package bashguard

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/dotfiles/claude-kit/go/internal/config"
	"github.com/ku5ic/dotfiles/claude-kit/go/internal/guard"
	"github.com/ku5ic/dotfiles/claude-kit/go/internal/hook"
	"github.com/ku5ic/dotfiles/claude-kit/go/internal/project"
)

const overlayAsk = "this writes the claude-kit overlay, which can switch the kit's own guards off; confirm the change"

// state is one command's evaluation: the first ask reason wins, and the
// directory tracks cd segments so a later push or install checks the right
// repo.
type state struct {
	h       *hook.Hook
	cfg     *config.Config
	home    string
	cwd     string
	prev    string // the cwd before the last cd, for cd -
	pending string
}

func (st *state) ask(reason string) {
	if st.pending == "" {
		st.pending = reason
	}
}

var (
	forkBomb    = regexp.MustCompile(`:\(\)[[:space:]]*\{`)
	deviceWrite = regexp.MustCompile(`>[[:space:]]*/dev/(sd|nvme|disk|rdisk)`)
	xargsRm     = regexp.MustCompile(`xargs[[:space:]]+((-[^[:space:]]+[[:space:]]+)*)rm[[:space:]]+-[a-zA-Z]*[rRfF]`)
	sqQuoted    = regexp.MustCompile(`'[^']*'`)
	dqQuoted    = regexp.MustCompile(`"[^"]*"`)
)

// Check is the PreToolUse hook for Bash.
func Check(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	cmd := h.Payload.String("tool_input.command")
	if cmd == "" {
		return nil
	}
	h.SetContext("Command: " + cmd)
	cfg := h.Config()
	if cfg == nil {
		cfg = &config.Config{}
	}
	home := os.Getenv("HOME")
	norm := normalize(cmd)

	// Whole-string checks, distinctive enough that quoted false positives
	// aren't realistic.
	full := []struct {
		re           *regexp.Regexp
		reason, rule string
	}{
		{forkBomb, "fork bomb pattern", "fork-bomb"},
		{deviceWrite, "write to raw disk device", "device-write"},
	}
	for _, f := range full {
		if f.re.MatchString(norm) {
			if err := h.Block(f.reason, f.rule); err != nil {
				return err
			}
		}
	}
	if re := rcRedirect(cfg, home); re != nil && re.MatchString(norm) {
		if err := h.Block("direct write to a shell rc file. Use the dotfiles repo.", "rc-redirect"); err != nil {
			return err
		}
	}
	// On a quote-stripped copy: a && or | inside a quoted literal isn't one.
	if xargsRm.MatchString(dqQuoted.ReplaceAllString(sqQuoted.ReplaceAllString(cmd, ""), "")) {
		if err := h.Block("xargs rm with recursive or force flag", "xargs-rm"); err != nil {
			return err
		}
	}

	cwd := h.Payload.String("cwd")
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	st := &state{h: h, cfg: cfg, home: home, cwd: resolveDir(home, "/", cwd)}
	// The raw command, not norm: <<- strips tabs only, and norm would turn a
	// tab-indented terminator into spaces that no longer close the heredoc.
	segs, unparsed := parseAll(cmd)
	if unparsed {
		st.ask("guard-bash could not parse part of this command, so it went unchecked; confirm it")
	}
	for _, seg := range segs {
		if err := st.segment(seg); err != nil {
			return err
		}
	}

	switch {
	case st.pending != "":
		h.Decide("ask", st.pending)
	case readonlyCall(cmd, norm):
		h.Decide("allow", "side-effect-free claude-kit script")
	}
	return nil
}

// rcRedirect matches a > or >> into a shell rc file under home.
func rcRedirect(cfg *config.Config, home string) *regexp.Regexp {
	var alts []string
	for _, rc := range cfg.RCFiles {
		alts = append(alts, regexp.QuoteMeta(strings.TrimPrefix(rc, "~/")))
	}
	if len(alts) == 0 {
		return nil
	}
	return regexp.MustCompile(`>+[[:space:]]*(\$HOME|\$\{HOME\}|~|` + regexp.QuoteMeta(home) + `)/(` + strings.Join(alts, "|") + `)([[:space:]]|$)`)
}

// resolveDir resolves dir (relative or ~-prefixed) against base, physically
// when it exists, so it compares equal to git's toplevel.
func resolveDir(home, base, dir string) string {
	if strings.HasPrefix(dir, "~") {
		dir = home + dir[1:]
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(base, dir)
	}
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		return real
	}
	return filepath.Clean(dir)
}

// Wrappers that only change how a command runs, and their options that take
// a separate value.
var (
	wrappers     = map[string]bool{"command": true, "env": true, "builtin": true, "exec": true, "nohup": true, "nice": true, "timeout": true, "stdbuf": true, "ionice": true, "chrt": true}
	wrapperValue = map[string]bool{"nice:-n": true, "env:-u": true, "env:-C": true, "exec:-a": true, "timeout:-s": true, "timeout:-k": true, "ionice:-c": true, "ionice:-n": true, "chrt:-p": true}
	assignment   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
)

// lead finds where the real command starts in a call's words, seeing
// through VAR=value words and command/env/nice/timeout-style wrappers with
// their own options.
func lead(words []Word) int {
	i := 0
	skipAssigns := func() {
		for i < len(words) && assignment.MatchString(words[i].Value) {
			i++
		}
	}
	skipAssigns()
	for i < len(words) && wrappers[words[i].Value] && i+1 < len(words) {
		w := words[i].Value
		i++
		for i < len(words)-1 && strings.HasPrefix(words[i].Value, "-") {
			opt := words[i].Value
			i++
			if opt == "--" {
				break
			}
			if wrapperValue[w+":"+opt] && i < len(words)-1 {
				i++
			}
		}
		if (w == "timeout" || w == "chrt") && i < len(words)-1 && words[i].Value != "" && words[i].Value[0] >= '0' && words[i].Value[0] <= '9' {
			i++
		}
		skipAssigns()
	}
	return i
}

var interpreters = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "fish": true, "node": true, "ruby": true, "perl": true}

// pipeToShell blocks a download piped into an interpreter later in the
// same pipeline (curl x | sh, wget -O- x | tee log | sudo bash). Parsed
// calls, not the raw string: "curl x | sh" as quoted text is data.
func (st *state) pipeToShell(seg Segment) error {
	fetched := false
	for _, call := range seg.Calls {
		words := call.Words[lead(call.Words):]
		if len(words) > 0 && baseName(words[0].Value) == "sudo" {
			words = words[1:]
			for len(words) > 0 && strings.HasPrefix(words[0].Value, "-") {
				words = words[1:]
			}
		}
		if len(words) == 0 {
			continue
		}
		name := baseName(words[0].Value)
		if fetched && (interpreters[name] || strings.HasPrefix(name, "python")) {
			return st.h.Block("piping network content into an interpreter", "pipe-to-shell")
		}
		if name == "curl" || name == "wget" {
			fetched = true
		}
	}
	return nil
}

// segment runs the per-command checks on every command of a pipeline.
func (st *state) segment(seg Segment) error {
	if err := st.pipeToShell(seg); err != nil {
		return err
	}
	for ci, call := range seg.Calls {
		if err := st.redirects(call); err != nil {
			return err
		}
		start := lead(call.Words)
		if start >= len(call.Words) {
			continue
		}
		// $(echo rm) -rf ~ names its command only at runtime, as does
		// sh <<< "$(echo rm -rf ~)" once its body is parsed.
		if raw := call.Words[start].Raw; strings.Contains(raw, "$(") || strings.Contains(raw, "`") {
			st.ask("the command name comes from a substitution, so guard-bash can't check it; confirm it")
		}
		c := command{st: st, seg: seg, call: ci, name: baseName(call.Words[start].Value), args: call.Words[start+1:], redirs: call.Redirs, inputs: call.Inputs, idx: start}
		c.rest = seg.Rest(ci, start)
		c.text = c.name + c.rest
		c.alone = len(seg.Calls) == 1
		if err := c.check(); err != nil {
			return err
		}
	}
	return nil
}

// redirects blocks a write to a shell rc file, and asks before a write to
// the overlay or loose into the current directory: a bare name or ./name
// lands in the repo root in a project session. A subdir target
// (docs/report.md) is plausibly a deliverable.
func (st *state) redirects(call Call) error {
	for _, r := range call.Redirs {
		target := r.Target.Value
		// The quote-removed target: the whole-string regex misses >> "$HOME/.zshrc".
		// '$HOME/.zshrc' is a literal name in cwd, not the rc file.
		if !strings.HasPrefix(r.Target.Raw, "'") && guard.IsRCFile(st.cfg, target) {
			if err := st.h.Block("direct write to a shell rc file. Use the dotfiles repo.", "rc-redirect"); err != nil {
				return err
			}
		}
		if st.isOverlayArg(target) {
			st.ask(overlayAsk)
		}
		if (!strings.Contains(target, "/") || strings.HasPrefix(target, "./")) && looseWriteTarget(target) && st.inWorktree() {
			st.ask("'> " + target + "' writes into the current directory; rules/tooling.md wants > \"$(scratch-dir.sh)/" + baseName(target) + "\". Confirm only if this file belongs in the project tree.")
		}
	}
	return nil
}

// inWorktree is true when the tracked cwd (after any cd) is in a git work
// tree and not under a scratch directory: cd /tmp or cd .claude/scratch
// makes a bare > name harmless.
func (st *state) inWorktree() bool {
	if st.cwd == "" || strings.Contains(st.cwd+"/", "/.claude/scratch/") {
		return false
	}
	return project.Toplevel(st.cwd) != ""
}

// looseWriteTarget is true for a relative target that would land loose in
// the repo instead of scratch. Unresolvable targets (variables,
// substitutions, fd duplications) are false: $(scratch-dir.sh) is the
// sanctioned form.
func looseWriteTarget(p string) bool {
	switch {
	case p == "", p == "-", strings.HasPrefix(p, "$"), strings.Contains(p, "$("),
		strings.HasPrefix(p, `"`), strings.HasPrefix(p, "'"), strings.HasPrefix(p, "&"), strings.HasPrefix(p, "("):
		return false
	case p == "scratch", strings.HasPrefix(p, "scratch/"), strings.HasSuffix(p, "/scratch"), strings.Contains(p, "/scratch/"):
		return false
	case strings.HasPrefix(p, "/"), strings.HasPrefix(p, "~"):
		return false
	}
	return true
}

// scratchTarget is true when a download target is stdout, /dev/null, an fd,
// or inside a .claude/scratch directory. $(scratch-dir.sh) counts; any
// other unexpanded variable, and any "..", doesn't, since it can't be
// checked.
func (st *state) scratchTarget(p string) bool {
	p = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(p, `"`), `'`), `"`)
	p = strings.TrimSuffix(p, `'`)
	if strings.Contains(p, "..") {
		return false
	}
	switch {
	case p == "-", p == "/dev/null", len(p) == 2 && p[0] == '&' && p[1] >= '0' && p[1] <= '9',
		p == "$(scratch-dir.sh)", strings.HasPrefix(p, "$(scratch-dir.sh)/"),
		p == "`scratch-dir.sh`", strings.HasPrefix(p, "`scratch-dir.sh`/"):
		return true
	}
	p = expandHome(st.home, p)
	if strings.ContainsAny(p, "$`") {
		return false
	}
	if !strings.HasPrefix(p, "/") {
		p = st.cwd + "/" + p
	}
	return strings.HasSuffix(p, "/.claude/scratch") || strings.Contains(p, "/.claude/scratch/")
}

func expandHome(home, p string) string {
	for _, prefix := range []string{"~", "$HOME", "${HOME}"} {
		if strings.HasPrefix(p, prefix) {
			return home + p[len(prefix):]
		}
	}
	return p
}

// isOverlayArg is true when a word (quoted, ~- or $HOME-prefixed, or
// relative to the segment's cwd) names the kit overlay. The basename test
// keeps path resolution off every other argument.
func (st *state) isOverlayArg(arg string) bool {
	arg = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(arg, `"`), `'`), `"`)
	arg = strings.TrimSuffix(arg, `'`)
	if arg != "claude-kit.local.yml" && !strings.HasSuffix(arg, "/claude-kit.local.yml") {
		return false
	}
	arg = expandHome(st.home, arg)
	if !strings.HasPrefix(arg, "/") {
		arg = st.cwd + "/" + arg
	}
	return guard.IsOverlay(st.h.Paths, arg)
}

// isProtected is true when ref names a protected branch, ignoring
// refs/heads/, refs/remotes/<remote>/, and origin/ prefixes.
func (st *state) isProtected(ref string) bool {
	ref = strings.TrimPrefix(ref, "refs/heads/")
	if rest, ok := strings.CutPrefix(ref, "refs/remotes/"); ok {
		if _, branch, found := strings.Cut(rest, "/"); found {
			ref = branch
		}
	}
	ref = strings.TrimPrefix(ref, "origin/")
	return slices.Contains(st.cfg.ProtectedBranches, ref)
}

// currentBranch of the repo a git command targets: -C resolved against the
// segment's cwd. Empty outside a repo or on a detached HEAD.
func (st *state) currentBranch(gitDir string) string {
	dir := expandHome(st.home, gitDir)
	switch {
	case dir == "":
		dir = st.cwd
	case !strings.HasPrefix(dir, "/"):
		dir = st.cwd + "/" + dir
	}
	out, err := exec.Command("git", "-C", dir, "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Kit scripts that only read state or create the scratch/plans directories.
// Plugins can't ship allow rules, so the hook allows them itself; settings
// deny and ask rules still win over a hook allow. run-checks.sh stays out:
// it runs project-defined scripts.
var readonlyScripts = []string{"scratch-dir.sh", "plans-dir.sh", "git-base.sh", "project-name.sh", "project-root.sh", "detect-stack.sh", "skills-report.sh", "blast-radius.sh"}

// readonlyCall is true for a lone kit script call: no chaining, pipes,
// redirects, or substitutions that could smuggle in a second command.
func readonlyCall(cmd, norm string) bool {
	if strings.ContainsAny(cmd, ";&|<>`\n") || strings.Contains(cmd, "$(") {
		return false
	}
	first, rest, _ := strings.Cut(norm, " ")
	if !slices.Contains(readonlyScripts, first) {
		return false
	}
	return first != "git-base.sh" || gitBaseFlagsSafe(strings.Fields(rest))
}

// gitBaseFlagsSafe is false when git-base.sh would hand git a flag that can
// write files (--output) or run programs (--ext-diff): only the flags the
// kit's own skills pass go through without a prompt.
func gitBaseFlagsSafe(words []string) bool {
	// The shell drops quotes, backslashes, and $'': '--output=x' and
	// \--output=x reach git as --output=x.
	unquote := strings.NewReplacer(`'`, "", `"`, "", `\`, "", "$", "")
	for _, w := range words {
		w = unquote.Replace(w)
		switch {
		case !strings.HasPrefix(w, "-"):
		case len(w) > 1 && w[1] >= '0' && w[1] <= '9':
		case slices.Contains([]string{"--diff", "--log", "--stat", "--name-only", "--name-status", "--no-merges", "--oneline", "--shortstat"}, w):
		default:
			return false
		}
	}
	return true
}

// nearestLockfile is project.NearestLockfile from a physical dir.
func (st *state) nearestLockfile(dir, ecosystem string) (project.Lockfile, bool) {
	return project.NearestLockfile(st.cfg, dir, ecosystem)
}
