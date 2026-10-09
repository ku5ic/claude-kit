package checks

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/tools"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// EditedFiles lists the file_path (or notebook_path) of every Edit, Write,
// MultiEdit, and NotebookEdit call in the last turn of a transcript: the
// entries after the last real user prompt. A tool result is not a prompt;
// neither is a meta entry.
func EditedFiles(path string) ([]string, error) {
	var edited []string
	err := transcript.Each(path, func(e transcript.Entry) {
		if e.StartsTurn() {
			edited = edited[:0]
			return
		}
		if e.Type != "assistant" {
			return
		}
		for _, b := range e.ToolUses() {
			if slices.Contains(transcript.EditTools, b.Name) {
				edited = append(edited, b.Path())
			}
		}
	})
	return edited, err
}

// Group is one planned check run: the files an adapter claims under one
// directory, which is where it runs, and the command (nil when the tool
// isn't installed, with Skip saying why).
type Group struct {
	Adapter tools.Adapter
	Dir     string
	Why     string
	// Derived is what the project's own invocation adds (Source says where
	// it came from); zero when none was found.
	Derived tools.Derived
	Files   []string
	Words   []string
	Bin     tools.Resolution // where Words' binary comes from
	Skip    string
	label   string
}

// Plan works out which adapters claim which of the edited files under root
// (relative paths resolved against base) and the command each would run,
// without running anything. Deleted, ignored, and outside-the-repo files
// don't count: an ignored file (scratch, build output) never lands in the
// repo, so its checks have nothing to protect.
func Plan(cfg *config.Config, root, base string, edited []string) []*Group {
	adapters := tools.All(cfg)
	var groups []*Group
	byKey := map[string]*Group{}
	seen := map[string]bool{}
	ignored := gitIgnored(root, edited, base)
	for _, path := range edited {
		if path == "" {
			continue
		}
		path = project.PhysicalPath(absUnder(base, path))
		if !strings.HasPrefix(path, root+"/") || seen[path] || !project.IsFile(path) || ignored[path] {
			continue
		}
		seen[path] = true
		for i, a := range adapters {
			claim, ok := a.Claims(path, root)
			if !ok {
				continue
			}
			key := fmt.Sprintf("%d|%s", i, claim.Dir)
			g, found := byKey[key]
			if !found {
				g = &Group{Adapter: a, Dir: claim.Dir, Why: claim.Why}
				byKey[key] = g
				groups = append(groups, g)
			}
			g.Files = append(g.Files, path)
		}
	}
	for _, g := range groups {
		g.label = fmt.Sprintf("%s (%d file%s)", g.Adapter.Name, len(g.Files), plural(len(g.Files)))
		g.label += project.SubLabel(project.Rel(root, g.Dir))
		if strings.TrimSpace(g.Adapter.Cmd) == "" {
			g.Skip = "no cmd in kit.yml"
			continue
		}
		mode := tools.Default
		if g.Adapter.LocalOnly {
			mode = tools.LocalOnly
		}
		res := tools.Resolve(cfg, g.Dir, root, g.Adapter.Bin, mode)
		if res.Words == nil {
			g.Skip = res.Skip
			continue
		}
		g.Bin = res
		g.Derived, _ = g.Adapter.Derive(cfg, g.Dir, root)
		g.Words = expand(g.Adapter.Cmd, res.Words, g.Files, g.Dir, g.Derived)
	}
	return groups
}

// Outcome is one FileChecks run.
type Outcome struct {
	Report   string // PASS/FAIL/SKIP lines
	Failures string // each failing tool's last maxOutputLines output lines
	Summary  string // "checks: N passed, N failed, N skipped"
	Failed   bool
}

// FileChecks runs the planned checks; nil when no check claimed a file.
func FileChecks(cfg *config.Config, root, base string, edited []string) *Outcome {
	groups := Plan(cfg, root, base, edited)
	if len(groups) == 0 {
		return nil
	}
	timeout := time.Duration(cfg.CheckTimeout) * time.Second
	results := runGroups(groups, timeout)
	t := tally{root: root, groups: groups}
	for i, g := range groups {
		if g.Skip == "" && results[i].timedOut {
			g.Skip = fmt.Sprintf("timed out after %s", timeout)
		}
		if g.Skip == "" {
			g.Skip = results[i].skip
		}
		t.record(g, &results[i])
	}
	return &Outcome{
		Report:   t.rep.String(),
		Failures: t.fails.String(),
		Summary:  summary(t.pass, t.fail, t.skip),
		Failed:   t.fail > 0,
	}
}

// runGroups runs every group that isn't skipped, in parallel.
func runGroups(groups []*Group, timeout time.Duration) []result {
	results := make([]result, len(groups))
	var wg sync.WaitGroup
	for i, g := range groups {
		if g.Skip == "" {
			wg.Go(func() {
				// A panic here would exit 2, which Claude Code reads as a
				// block; hook.RunCheck's recover can't reach this goroutine.
				defer func() {
					if recover() != nil {
						results[i] = result{skip: "crashed; failing open"}
					}
				}()
				results[i] = runGroup(g, timeout)
			})
		}
	}
	wg.Wait()
	return results
}

// tally builds the report: a line per group, and for a block, each failing
// tool's output and every skip's reason.
type tally struct {
	root             string
	groups           []*Group
	rep, fails       strings.Builder
	pass, fail, skip int
	changed          map[string]map[int]bool
	changedKnown     bool
}

func (t *tally) record(g *Group, res *result) {
	if g.Skip != "" {
		line := skipLine(g.label, g.Skip)
		t.rep.WriteString(line)
		// Also in a block's message, so the skipped count has its reasons.
		t.fails.WriteString(line)
		t.skip++
		return
	}
	bin := g.Bin.BinLine()
	if res.err == nil {
		fmt.Fprintf(&t.rep, "PASS %s\n%s", g.label, bin)
		t.pass++
		return
	}
	if !t.changedKnown {
		t.changed, t.changedKnown = changedLines(t.root, "HEAD", planned(t.groups)), true
	}
	blocking, old, ok := newFindings(g, res.out.String(), t.root, t.changed)
	if ok && len(blocking) == 0 {
		fmt.Fprintf(&t.rep, "PASS %s (%d finding%s on unchanged lines)\n%s", g.label, old, plural(old), bin)
		t.pass++
		return
	}
	fmt.Fprintf(&t.rep, "FAIL %s\n%s", g.label, bin)
	t.fail++
	if ok {
		fmt.Fprintf(&t.fails, "FAIL %s\n%s%s\n", g.label, bin, strings.Join(head(blocking), "\n"))
		if old > 0 {
			fmt.Fprintf(&t.fails, "(%d more on unchanged lines don't block)\n", old)
		}
		return
	}
	// The tail: linters print findings and the summary last, after
	// preambles like rubocop's unconfigured-cops notice.
	lines := strings.Split(strings.TrimRight(res.out.String(), "\n"), "\n")
	fmt.Fprintf(&t.fails, "FAIL %s\n%s%s\n", g.label, bin, strings.Join(lines[max(0, len(lines)-maxOutputLines):], "\n"))
}

type result struct {
	out      bytes.Buffer
	err      error
	timedOut bool
	skip     string
}

// runGroup runs one check in its own process group, so a timeout kills the
// workers a test runner spawned too, not only the runner.
func runGroup(g *Group, timeout time.Duration) result {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var res result
	cmd := exec.CommandContext(ctx, g.Words[0], g.Words[1:]...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = g.Dir, &res.out, &res.out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	// A killed group's orphans could hold the output pipe open.
	cmd.WaitDelay = 2 * time.Second
	res.err = cmd.Run()
	res.timedOut = ctx.Err() != nil
	return res
}

// newFindings splits a failed check's parsed findings into those on lines
// the working tree changed (their text, blocking) and a count of the rest.
// ok is false when the output can't be trusted to list them all: no
// parser, nothing parsed, or findings left out.
func newFindings(g *Group, out, root string, changed map[string]map[int]bool) (blocking []string, old int, ok bool) {
	if g.Adapter.Findings == nil {
		return nil, 0, false
	}
	found, complete := g.Adapter.Findings.Parse(out)
	if !complete || len(found) == 0 {
		return nil, 0, false
	}
	for _, f := range found {
		file := editedFile(f.File, g)
		touched := file != "" && onChangedLine(changed, file, f.Line)
		if !touched {
			old++
			continue
		}
		blocking = append(blocking, strings.ReplaceAll(f.Text, root+"/", ""))
	}
	return blocking, old, true
}

// editedFile is the group's edited file a tool's path names: relative to
// where it ran, absolute, or relative to somewhere else (golangci-lint v2
// prints paths relative to its config), matched by suffix. "" for a file
// the turn didn't edit.
func editedFile(path string, g *Group) string {
	path = absUnder(g.Dir, path)
	if slices.Contains(g.Files, path) {
		return path
	}
	if physical := project.PhysicalPath(path); slices.Contains(g.Files, physical) {
		return physical
	}
	if project.IsFile(path) {
		return ""
	}
	for _, f := range g.Files {
		if strings.HasSuffix(f, "/"+strings.TrimPrefix(path, g.Dir+"/")) {
			return f
		}
	}
	return ""
}

func planned(groups []*Group) []string {
	var files []string
	for _, g := range groups {
		for _, f := range g.Files {
			if !slices.Contains(files, f) {
				files = append(files, f)
			}
		}
	}
	return files
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// gitIgnored asks git once which of the edited files under root it ignores,
// keyed by physical path. Any failure means "none ignored": check them all.
func gitIgnored(root string, edited []string, base string) map[string]bool {
	var paths []string
	for _, p := range edited {
		if p == "" {
			continue
		}
		// A path outside the repo is fatal to check-ignore, dropping every
		// path after it.
		if p = project.PhysicalPath(absUnder(base, p)); strings.HasPrefix(p, root+"/") {
			paths = append(paths, p)
		}
	}
	ignored := map[string]bool{}
	if len(paths) == 0 {
		return ignored
	}
	cmd := git.Command(root, "check-ignore", "--stdin")
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\n") + "\n")
	out, _ := cmd.Output() // exit 1 means none ignored
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			ignored[line] = true
		}
	}
	return ignored
}

// expand fills a check's cmd word by word: {bin} (a whole word) becomes the
// resolved run words, a word holding {files} repeats once per file, and a
// word holding {dirs} once per file directory as ./<relative to dir>. The
// project's derived env goes first (through env), and its derived flags
// just before the files, skipping any the template already passes.
func expand(cmd string, bin, files []string, dir string, derived tools.Derived) []string {
	var parts []string
	if len(derived.Env) > 0 {
		parts = append([]string{"env"}, derived.Env...)
	}
	template := strings.Fields(cmd)
	flagsAdded := false
	addFlags := func() {
		if flagsAdded {
			return
		}
		flagsAdded = true
		for _, flag := range derived.Flags {
			if !slices.Contains(template, flag) {
				parts = append(parts, flag)
			}
		}
	}
	for _, word := range template {
		if strings.Contains(word, "{files}") || strings.Contains(word, "{dirs}") {
			addFlags()
		}
		switch {
		case strings.Contains(word, "{files}"):
			for _, f := range files {
				parts = append(parts, strings.ReplaceAll(word, "{files}", f))
			}
		case strings.Contains(word, "{dirs}"):
			var dirs []string
			for _, f := range files {
				rel := filepath.Dir("./" + strings.TrimPrefix(f, dir+"/"))
				if rel != "." {
					rel = "./" + rel
				}
				if !slices.Contains(dirs, rel) {
					dirs = append(dirs, rel)
				}
			}
			for _, d := range dirs {
				parts = append(parts, strings.ReplaceAll(word, "{dirs}", d))
			}
		case word == "{bin}":
			parts = append(parts, bin...)
		default:
			parts = append(parts, word)
		}
	}
	addFlags()
	return parts
}
