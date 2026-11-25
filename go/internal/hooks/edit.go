// Package hooks implements each Claude Code hook as a hook.Check.
package hooks

import (
	"bytes"
	"cmp"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/sources"
	"github.com/ku5ic/claude-kit/go/internal/stackctx"
)

var (
	gitDir      = regexp.MustCompile(`/\.git/`)
	ciWorkflows = regexp.MustCompile(`\.github/workflows/.*\.ya?ml$`)
)

// GuardEdit blocks reads and writes of credential files and writes to other
// risky paths, regardless of permission rules. Plugins can't ship
// settings.json deny rules, so for a plugin install this is the only
// credential-read protection. PreToolUse for Read, Grep, Edit, Write,
// MultiEdit.
func GuardEdit(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	// Without kit.yml nothing is configured: nothing matches, every check fails open.
	cfg := cmp.Or(h.Config(), &config.Config{})
	tool := h.Payload.String(hook.KeyToolName)
	if tool == "Grep" {
		return guardSearch(h, cfg)
	}
	path := h.Payload.FilePath()
	if path == "" {
		return nil
	}
	return guardPath(h, cfg, tool, path, tool == "Read")
}

// GuardWrites gives each path a Bash command writes the path rules an Edit
// gets. bashguard finds the paths; the two meet in cmd/kit.
func GuardWrites(h *hook.Hook, paths []string) error {
	cfg := cmp.Or(h.Config(), &config.Config{})
	for _, p := range paths {
		if err := guardPath(h, cfg, "Bash", p, false); err != nil {
			return err
		}
	}
	return nil
}

// guardPath is guard-edit's rules for one path tool reads, or else writes:
// a credential file blocks either way, and a write to a lockfile, .git, an
// rc file, a CI workflow, or a kit switch blocks or asks.
func guardPath(h *hook.Hook, cfg *config.Config, tool, path string, reading bool) error {
	h.SetContext(layout.KV("path", path))
	if guard.IsSensitive(cfg, h.Home, path) {
		reason, rule := "writing a credential or key file is not permitted", "sensitive-write"
		if reading {
			reason, rule = "reading a credential or key file is not permitted", "sensitive-read"
		}
		if err := h.Block(reason, rule); err != nil {
			return err
		}
	}

	// Everything below guards writes only.
	if reading {
		return nil
	}
	if guard.IsGuardedLockfile(cfg, path) && !project.IsScratch(h.Paths, path) {
		if err := h.Block("lockfile edit\n"+layout.KV("do", "use the package manager"), "lockfile-edit"); err != nil {
			return err
		}
	}
	if gitDir.MatchString(path) {
		if err := h.Block("edit inside .git/", "git-dir-edit"); err != nil {
			return err
		}
	}
	if guard.IsRCFile(cfg, h.Home, path) {
		if err := h.Block("direct edit to a shell rc file\n"+layout.KV("do", "use the dotfiles repo"), "rc-edit"); err != nil {
			return err
		}
	}
	if ciWorkflows.MatchString(path) {
		h.Decide("ask", "this is a CI workflow, which changes what runs on every push; confirm the change")
	} else if tasks := gateTasks(h, cfg, path); len(tasks) > 0 {
		h.Decide("ask", "this edit changes what kit's gates run at Stop and in the full gate, with no permission prompt: "+strings.Join(tasks, ", ")+"; confirm the change")
	}
	if what := guard.KitSwitch(h.Paths, h.Payload.Cwd(), path); what != "" {
		h.Decide("ask", guard.KitSwitchAsk(what))
	}
	if tool == "Write" && newRootArtifact(cfg, path) {
		h.Decide("ask", "a new "+filepath.Base(path)+" at the repo root; skills/kit-rules/reference/output.md section 3 puts reports and logs in `kit scratch-dir`. Confirm only if it belongs in the repo")
	}
	return nil
}

// guardSearch blocks a Grep that can read a credential file. Its root is
// tool_input.path, else cwd, which is also what a relative path joins. Only
// a path given blocks for a "~/" entry under it: a pathless Grep in ~ is the
// common case, not a reach for ~/.ssh.
func guardSearch(h *hook.Hook, cfg *config.Config) error {
	root := searchRoot(h)
	glob := h.Payload.String(hook.KeyInputGlob)
	h.SetContext(layout.KV("path", root) + "\n" + layout.KV("glob", glob))
	if guard.SearchesSensitive(cfg, h.Home, root, glob, h.Payload.String(hook.KeyInputPath) != "") {
		return h.Block("searching a credential or key file is not permitted", "sensitive-read")
	}
	// With a glob that matches every basename, a loose .env or *.pem under
	// root is searched. Inside the project, where Grep needs no prompt,
	// exclude them; the allow keeps auto mode from denying the rewrite.
	// Outside, the gap stays, and a payload with no tool_input has nothing
	// to rewrite.
	input := h.Payload.Object(hook.KeyToolInput)
	if excluded, ok := withExclusion(glob, guard.SensitiveExclusion(cfg)); ok && root != "" && input != nil {
		if top, anchored := project.Root(h.Payload.Cwd()); anchored && fsx.Within(top, fsx.PhysicalPath(filepath.Clean(root))) {
			input = maps.Clone(input)
			input["glob"] = excluded
			h.UpdateInput(input)
			h.Decide("allow", "kit excluded credential files from a Grep inside the project")
		}
	}
	return nil
}

// withExclusion is glob with exclude ("!{.env,*.pem}") applied, and
// whether glob matched a credential basename that needs it: no glob, one
// of only * and /, or a brace alternative of them stands for exclude; a
// negative glob gets exclude's entries beside its own alternatives.
func withExclusion(glob, exclude string) (string, bool) {
	entries, ok := strings.CutPrefix(exclude, "!{")
	if !ok {
		return "", false
	}
	entries = strings.TrimSuffix(entries, "}")
	if negated, ok := strings.CutPrefix(glob, "!"); ok {
		own, ok := alternatives(negated)
		return "!{" + strings.Join(own, ",") + "," + entries + "}", ok && negated != ""
	}
	matchesAll := func(g string) bool { return strings.Trim(g, "*/") == "" }
	if alternatives, ok := strings.CutPrefix(glob, "{"); ok {
		return exclude, slices.ContainsFunc(strings.Split(strings.TrimSuffix(alternatives, "}"), ","), matchesAll)
	}
	return exclude, matchesAll(glob)
}

// alternatives is glob with its brace groups expanded, so they can share
// one {...} with other entries: *.{js,ts} is *.js and *.ts. A nested group,
// a stray brace, or a comma outside a group isn't expanded: ok is false.
func alternatives(glob string) (out []string, ok bool) {
	open := strings.IndexByte(glob, '{')
	if open < 0 {
		return []string{glob}, !strings.ContainsAny(glob, "},")
	}
	end := strings.IndexByte(glob[open:], '}') + open
	if end < open || strings.ContainsAny(glob[:open], "},") || strings.Contains(glob[open+1:end], "{") {
		return nil, false
	}
	rests, ok := alternatives(glob[end+1:])
	if !ok {
		return nil, false
	}
	for alt := range strings.SplitSeq(glob[open+1:end], ",") {
		for _, rest := range rests {
			out = append(out, glob[:open]+alt+rest)
		}
	}
	return out, true
}

// searchRoot is where a Grep searches: tool_input.path, else cwd, which is
// also what a relative path joins.
func searchRoot(h *hook.Hook) string {
	cwd := h.Payload.Cwd()
	root := guard.ExpandHome(h.Home, cmp.Or(h.Payload.String(hook.KeyInputPath), cwd))
	if root != "" && !filepath.IsAbs(root) {
		root = filepath.Join(cwd, root)
	}
	return root
}

// newRootArtifact is true for a file that doesn't exist yet, directly in a
// repo's toplevel, with a root_artifact_exts extension.
func newRootArtifact(cfg *config.Config, path string) bool {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	if !slices.Contains(cfg.RootArtifactExts, ext) || fsx.IsFile(path) {
		return false
	}
	dir := fsx.PhysicalPath(filepath.Dir(path))
	return dir == git.Toplevel(dir)
}

// gateTasks is each task, as "file: name", that this edit of path adds to
// a file the gates read, or gives another command. A Bash write has no
// content to compare, so it names none.
func gateTasks(h *hook.Hook, cfg *config.Config, path string) []string {
	dir := fsx.PhysicalPath(filepath.Dir(path))
	root := git.Toplevel(dir)
	if root == "" {
		return nil
	}
	rel, err := filepath.Rel(root, filepath.Join(dir, filepath.Base(path)))
	if err != nil || !sources.Reads(rel) {
		return nil
	}
	before, after, ok := written(h.Payload, path)
	if !ok {
		return nil
	}
	var out []string
	for _, name := range sources.ChangedTasks(cfg, rel, before, after) {
		out = append(out, filepath.ToSlash(rel)+": "+name)
	}
	return out
}

// GuardSkills blocks edits of mapped file types until the patterns skill
// skill_file_map requires is loaded this session: one extra round trip per
// skill set per session, by design. Reads are never gated.
func GuardSkills(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	if !slices.Contains(filePathTools, h.Payload.String(hook.KeyToolName)) {
		return nil
	}
	if missing := unloadedSkills(h, []string{h.Payload.FilePath()}, nil); len(missing) > 0 {
		h.Deny("this edit needs "+strings.Join(missing, ", ")+" loaded\n"+layout.KV("do", "load it via the Skill tool, then retry the edit"), "skills-gate")
	}
	return nil
}

// unloadedSkills is the skills skill_file_map gives paths that this session
// hasn't loaded, less each skip reports true for, read from the skills log
// once; none for a scratch file or a session the cache can't name.
func unloadedSkills(h *hook.Hook, paths []string, skip func(skill string) bool) []string {
	session := h.Payload.SessionID()
	cfg := h.Config()
	if h.Paths.SessionFile(cache.SkillsLoaded, session) == "" || cfg == nil {
		return nil
	}
	marker := func(skill string) string { return h.Paths.SessionFile(cache.SkillsLoaded, session, skill) }
	var toCheck []string
	for _, path := range paths {
		if path == "" || project.IsScratch(h.Paths, path) {
			continue
		}
		for _, skill := range stackctx.FileMapSkills(cfg.SkillFileMap, path) {
			if slices.Contains(toCheck, skill) || skip != nil && skip(skill) {
				continue
			}
			if _, err := os.Stat(marker(skill)); err != nil {
				toCheck = append(toCheck, skill)
			}
		}
	}
	if len(toCheck) == 0 {
		return nil
	}
	return notLoaded(h, session, toCheck, marker)
}

// notLoaded is the skills of toCheck the session's skills log doesn't show
// loaded; each it does show gets its marker, so the log is read once per
// skill. A missing or unreadable log fails open: none.
func notLoaded(h *hook.Hook, session string, toCheck []string, marker func(string) string) []string {
	// A subagent's load counts for the whole session here.
	loaded, err := loadedSkills(h.Paths.LogFile(kitlog.Skills), session, nil)
	if err != nil {
		return nil
	}
	missing := notIn(h, loaded, toCheck)
	for _, skill := range toCheck {
		if !slices.Contains(missing, skill) {
			_ = fsx.WriteAtomic(marker(skill), nil, 0o644)
		}
	}
	return missing
}

// loadedSkills streams skills.jsonl once for session's skill_file values,
// skipping inject-context's required-skill and suggested-skill markers:
// those mean "surfaced", not "loaded". With byAgent set, only lines whose
// agent_id it accepts count, and none before their last compaction; "" is
// the main session.
func loadedSkills(logPath, session string, byAgent func(string) bool) (map[string]bool, error) {
	if _, err := os.Stat(logPath); err != nil {
		return nil, err
	}
	loaded := map[string]bool{}
	err := eachSkillEntry(logPath, session, func(entry kitlog.Entry) {
		if byAgent != nil {
			if !byAgent(entry.AgentID) {
				return
			}
			if entry.Event == kitlog.EventCompacted {
				clear(loaded)
			}
		}
		if entry.SkillFile != nil && entry.Loads() {
			loaded[*entry.SkillFile] = true
			// A plugin's skills log as <plugin>:<skill>; kit.yml maps bare names.
			if _, skill, ok := strings.Cut(*entry.SkillFile, ":"); ok && !strings.Contains(skill, "/") {
				loaded[skill] = true
			}
		}
	})
	if err != nil {
		return nil, err
	}
	return loaded, nil
}

// eachSkillEntry streams the skills log's lines for session to fn, in
// order. The log is every session's, so a line is decoded only once it
// holds the session's id.
func eachSkillEntry(logPath, session string, fn func(kitlog.Entry)) error {
	quoted := []byte(`"` + session + `"`)
	return kitlog.Each(logPath, func(line []byte) {
		var entry kitlog.Entry
		if !bytes.Contains(line, quoted) || json.Unmarshal(line, &entry) != nil || entry.SessionID == nil || *entry.SessionID != session {
			return
		}
		fn(entry)
	})
}

// contentChecks judge what a write adds, for an Edit before it lands and
// for a Bash command's changes after (bash-edits).
var contentChecks = []hook.NamedCheck{
	{Name: "guard-slop", Check: GuardSlop}, {Name: "guard-weaken", Check: GuardWeaken},
	{Name: "guard-imports", Check: GuardImports}, {Name: "guard-tokens", Check: GuardTokens},
}

// filePathTools are transcript.EditTools minus NotebookEdit, the tools
// whose new text written reads; a notebook cell gets the path rules alone.
var filePathTools = []string{"Edit", "Write", "MultiEdit"}

// GuardDispatch runs guard-edit's, guard-slop's, guard-weaken's,
// guard-imports', guard-tokens', guard-skills' with guard_skills on,
// skills-nudge's on a read or with guard_skills off, adr-nudge's, and then check-size's
// and verify-claims' checks against one payload read, each isolated so one
// failing open never skips the others. The skills gate is opt-in: blocking
// edits until a patterns skill loads is a personal policy, not a default.
func GuardDispatch(h *hook.Hook) int {
	checks := []hook.NamedCheck{{Name: "guard-edit", Check: GuardEdit}}
	edit := slices.Contains(filePathTools, h.Payload.String(hook.KeyToolName))
	if edit {
		checks = append(checks, contentChecks...)
	}
	enforced := stackctx.SkillsEnforced(h.Config())
	if enforced {
		checks = append(checks, hook.NamedCheck{Name: "guard-skills", Check: GuardSkills})
	}
	// The gate's deny already names the skill an edit needs.
	if !enforced || !edit {
		checks = append(checks, hook.NamedCheck{Name: "skills-nudge", Check: SkillsNudge})
	}
	checks = append(checks, hook.NamedCheck{Name: "adr-nudge", Check: ADRNudge})
	// Last, so a block from a cheap check saves its model run.
	checks = append(checks, hook.NamedCheck{Name: "check-size", Check: CheckSize}, hook.NamedCheck{Name: "verify-claims", Check: VerifyClaims})
	return hook.Run(h, checks...)
}
