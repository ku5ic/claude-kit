package hooks

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/weaken"
)

// sheetLimit caps a review sheet's lines under its headline; past it, the
// rest go to a scratch file the last line names.
const sheetLimit = 10

// emptyTree is git's tree with no files: the base before a repo's first
// commit.
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

var (
	// configFiles are the project configs and manifests a sheet points at
	// first, by base name.
	configFiles = []string{".env*", "tsconfig*.json", "eslint.config.*", "prettier.config.*", "next.config.*", "vite.config.*", "package.json", "go.mod", "pyproject.toml", "Cargo.toml", "kit.yml"}
	anyStep     = regexp.MustCompile(`(?m)^\s*[-*] \[[ xX]\]`)
	allowMarker = regexp.MustCompile(`kit: allow\b`)
)

// fileChange is one file a step changed, its lines added and removed.
type fileChange struct {
	path     string
	add, del int
	deleted  bool
}

// treeChanges is each file whose content differs between trees from and to,
// in path order; a binary file counts no lines.
func treeChanges(root, from, to string) ([]fileChange, error) {
	out, err := git.Output(root, "diff-tree", "-r", "-z", "--numstat", "--no-renames", from, to)
	if err != nil {
		return nil, err
	}
	deleted, err := git.Paths(root, "diff-tree", "-r", "-z", "--name-only", "--no-renames", "--diff-filter=D", from, to)
	if err != nil {
		return nil, err
	}
	var changes []fileChange
	for rec := range strings.SplitSeq(out, "\x00") {
		fields := strings.SplitN(rec, "\t", 3)
		if len(fields) != 3 {
			continue
		}
		add, _ := strconv.Atoi(fields[0])
		del, _ := strconv.Atoi(fields[1])
		changes = append(changes, fileChange{fields[2], add, del, slices.Contains(deleted, fields[2])})
	}
	return changes, nil
}

// lookReasons are the facts about c that put it first on a sheet, from git
// alone: what kind of file it is, and what its diff adds or removes.
func lookReasons(cfg *config.Config, root, from, to, plan string, c fileChange) []string {
	var why []string
	base := filepath.Base(c.path)
	switch {
	case guard.IsGuardedLockfile(cfg, c.path):
		why = append(why, "lockfile")
	case guard.GlobAny(configFiles, base):
		why = append(why, "config file")
	case ciWorkflows.MatchString("/"+c.path) || base == ".gitlab-ci.yml":
		why = append(why, "CI workflow")
	}
	if c.deleted {
		why = append(why, "deleted")
	}
	before, _ := git.Output(root, "cat-file", "-p", from+":"+c.path)
	after, _ := git.Output(root, "cat-file", "-p", to+":"+c.path)
	if len(allowMarker.FindAllString(after, -1)) > len(allowMarker.FindAllString(before, -1)) {
		why = append(why, `"kit: allow" marker added`)
	}
	w := weaken.Check(cfg.WeakenedChecks, c.path, before, after)
	if w.Lost > 0 {
		noun := "assertions"
		if w.Lost == 1 {
			noun = "assertion"
		}
		why = append(why, fmt.Sprintf("%d %s removed", w.Lost, noun))
	}
	for _, hit := range w.Hits {
		if hit.Reason == "" {
			why = append(why, hit.Kind+" added")
			break
		}
	}
	if plan != "" && !strings.Contains(plan, base) {
		why = append(why, "the plan doesn't name this file")
	}
	return why
}

// sheetGroup is a review sheet's files that share a reason to look, or
// the rest, under one heading.
type sheetGroup struct {
	reasons int
	head    string
	files   []string
}

// lookGroups groups changes by their reasons to look, files with more
// reasons first, then the rest with their line counts.
func lookGroups(cfg *config.Config, root, from, to, plan string, changes []fileChange) []*sheetGroup {
	width := 0
	for _, c := range changes {
		width = max(width, len(c.path))
	}
	var groups []*sheetGroup
	byHead := map[string]*sheetGroup{}
	rest := &sheetGroup{head: "other files"}
	for _, c := range changes {
		why := lookReasons(cfg, root, from, to, plan, c)
		if len(why) == 0 {
			rest.files = append(rest.files, fmt.Sprintf("%-*s  +%d -%d", width, c.path, c.add, c.del))
			continue
		}
		head := strings.Join(why, "; ")
		if byHead[head] == nil {
			byHead[head] = &sheetGroup{reasons: len(why), head: head}
			groups = append(groups, byHead[head])
		}
		byHead[head].files = append(byHead[head].files, c.path)
	}
	slices.SortStableFunc(groups, func(a, b *sheetGroup) int { return b.reasons - a.reasons })
	if len(rest.files) > 0 {
		groups = append(groups, rest)
	}
	return groups
}

// sheet is the review sheet for the change from tree from to tree to: a
// headline, then lookGroups, each heading counted. nil when nothing
// changed. File lines past sheetLimit go to the file more names, and the
// last line says so.
func sheet(cfg *config.Config, root, from, to, plan, title, more string) []string {
	changes, err := treeChanges(root, from, to)
	if err != nil || len(changes) == 0 {
		return nil
	}
	add, del := 0, 0
	for _, c := range changes {
		add, del = add+c.add, del+c.del
	}
	groups := lookGroups(cfg, root, from, to, plan, changes)
	// render lays out the groups, showing at most limit files; the rest
	// come back as spill lines.
	render := func(limit int) (lines, spill []string) {
		lines, shown := []string{fmt.Sprintf("kit review: %s, %d files, +%d -%d", title, len(changes), add, del), ""}, 0
		for _, g := range groups {
			// A group with no file left to show is only in the spill.
			if shown < limit {
				lines = append(lines, fmt.Sprintf("  %s (%d)", g.head, len(g.files)))
			}
			for _, f := range g.files {
				if shown < limit {
					lines, shown = append(lines, "    "+f), shown+1
					continue
				}
				spill = append(spill, g.head+"  "+f)
			}
		}
		return lines, spill
	}
	if len(changes) > sheetLimit && more != "" {
		lines, spill := render(sheetLimit - 1)
		if fsx.WriteAtomic(more, []byte(strings.Join(spill, "\n")+"\n"), 0o600) == nil {
			return append(lines, "  "+layout.KV("more", fmt.Sprintf("%d files in %s", len(spill), more)))
		}
	}
	lines, _ := render(len(changes))
	return lines
}

// reviewSheets shows the user a review sheet for each plan step this turn
// finished. Under step_pause step, that's the active plan's first open
// step, as the tree against HEAD: each step is committed before the next.
// Otherwise it's each step ticked since the last sheet, between the
// snapshots planTick took.
func reviewSheets(h *hook.Hook, cfg *config.Config, cwd, root string) {
	_, plan := activePlan(h, root)
	if plan == nil {
		return
	}
	more := func(label string) string {
		dir, err := project.ScratchDir(h.Paths, cwd, !h.DryRun)
		if err != nil || h.DryRun {
			return ""
		}
		return filepath.Join(dir, "kit-review-"+strings.ReplaceAll(label, " ", "-")+".txt")
	}
	total := len(anyStep.FindAll(steps(plan), -1))
	if cfg.Pause() == "step" {
		loc := openStep.FindIndex(steps(plan))
		if loc == nil {
			return
		}
		n := len(anyStep.FindAll(steps(plan)[:loc[0]], -1)) + 1
		from, err := git.Line(root, "rev-parse", "--verify", "--quiet", "HEAD^{tree}")
		if err != nil {
			from = emptyTree
		}
		to, err := git.Snapshot(root)
		if err != nil {
			return
		}
		label := fmt.Sprintf("step %d of %d", n, total)
		if lines := sheet(cfg, root, from, to, string(plan), label, more(label)); lines != nil {
			h.Notify(strings.Join(lines, "\n"))
		}
		return
	}
	ticks := planFile(h, cache.PlanTicks, root)
	data, err := os.ReadFile(ticks)
	if err != nil {
		return
	}
	marks := tickMarks(string(data))
	for i := 1; i < len(marks); i++ {
		label := fmt.Sprintf("step %d of %d", marks[i].done, total)
		if marks[i].done-marks[i-1].done > 1 {
			label = fmt.Sprintf("steps %d-%d of %d", marks[i-1].done+1, marks[i].done, total)
		}
		if lines := sheet(cfg, root, marks[i-1].tree, marks[i].tree, string(plan), label, more(label)); lines != nil {
			h.Notify(strings.Join(lines, "\n"))
		}
	}
	if len(marks) > 1 && !h.DryRun {
		// The last mark is the next sheet's base.
		_ = fsx.WriteAtomic(ticks, []byte(marks[len(marks)-1].String()), 0o600)
	}
}

// tickMark is the work tree when a plan had done steps ticked.
type tickMark struct {
	done int
	tree string
}

func (m tickMark) String() string { return fmt.Sprintf("%d %s\n", m.done, m.tree) }

// tickMarks reads a plan-ticks file, one "<done> <tree>" line per mark.
func tickMarks(data string) []tickMark {
	var marks []tickMark
	for line := range strings.Lines(data) {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if done, err := strconv.Atoi(fields[0]); err == nil {
			marks = append(marks, tickMark{done, fields[1]})
		}
	}
	return marks
}

// markTicks appends a mark to the session's plan-ticks file when plan has
// more done steps than its last mark, or starts the file when it has none.
// Only a batched pause reads the marks, so step_pause step writes none.
func markTicks(h *hook.Hook, root string, plan []byte) {
	if cfg := h.Config(); cfg == nil || cfg.Pause() == "step" || h.DryRun {
		return
	}
	file := planFile(h, cache.PlanTicks, root)
	if file == "" {
		return
	}
	data, _ := os.ReadFile(file)
	marks := tickMarks(string(data))
	done := len(doneStep.FindAll(steps(plan), -1))
	if len(marks) > 0 && done <= marks[len(marks)-1].done {
		return
	}
	tree, err := git.Snapshot(root)
	if err != nil {
		return
	}
	_ = fsx.WriteAtomic(file, []byte(string(data)+tickMark{done, tree}.String()), 0o600)
}

// PlanTick is the PostToolUse step on an edit: when the file is the
// session's active plan, it marks the work tree for each step ticked, so a
// batched pause can show one review sheet per step. With no active plan, a
// picked-up one becomes it.
func PlanTick(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return nil
	}
	// The plan's own work tree, or for a plan outside one the session's: the
	// one its approval keyed the marker by. ~/.claude/plans counts as
	// outside even when it's under git, as in a dotfiles repo.
	path := h.Payload.FilePath()
	// Only a .md becomes an active plan (activatePlan's callers), so any
	// other edit ends here, before git runs.
	if filepath.Ext(path) != ".md" {
		return nil
	}
	root := ""
	if !fsx.Within(h.Paths.PlansHome(), path) {
		root = git.Toplevel(filepath.Dir(path))
	}
	if root == "" {
		root = git.Toplevel(h.Payload.Cwd())
	}
	marker := planFile(h, cache.PlanActive, root)
	if root == "" || marker == "" {
		return nil
	}
	active, err := os.ReadFile(marker)
	if errors.Is(err, fs.ErrNotExist) && pickedUp(h, path) {
		activatePlan(h, root, path)
		return nil
	}
	if err != nil || string(active) != path {
		return nil
	}
	plan, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	markTicks(h, root, plan)
	return nil
}
