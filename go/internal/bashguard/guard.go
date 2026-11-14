package bashguard

import (
	"cmp"
	"iter"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitcmd"
	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// dirs is where a command line's next command runs: cwd, which each cd
// moves, and prev, the cwd before the last cd, for cd -.
type dirs struct {
	home, cwd, prev string
}

// tracks is true when cd with args moves d to a directory d can name: not
// with an option, nor cd - before any cd.
func (d *dirs) tracks(args []Word) bool {
	switch {
	case len(args) == 0:
		return true
	case args[0].Value == "-":
		return d.prev != ""
	default:
		return !strings.HasPrefix(args[0].Value, "-")
	}
}

// cd moves d as cd with args would.
func (d *dirs) cd(args []Word) {
	if !d.tracks(args) {
		return
	}
	prev := d.cwd
	switch {
	case len(args) == 0:
		d.cwd = d.home
	case args[0].Value == "-":
		d.cwd = d.prev
	default:
		d.cwd = resolveDir(d.home, d.cwd, args[0].Value)
	}
	d.prev = prev
}

// walked is one parsed call as walk yields it: its segment, the call, the
// index of its real command word, that command's base name, and the
// directory it runs in.
type walked struct {
	seg  Segment
	call Call
	i    int
	name string
	cwd  string
}

// walk yields each call of segs with a command word, run from cwd, a cd
// moving the directory for the calls after it.
func walk(segs []Segment, home, cwd string) iter.Seq[walked] {
	return func(yield func(walked) bool) {
		d := dirs{home: home, cwd: cwd}
		for _, seg := range segs {
			for _, call := range seg.Calls {
				i := lead(call.Words)
				if i >= len(call.Words) {
					continue
				}
				name := baseName(call.Words[i].Value)
				// In a pipeline or a subshell, cd moves nothing after it.
				if name == "cd" && len(seg.Calls) == 1 && !seg.subshell {
					d.cd(call.Words[i+1:])
				}
				if !yield(walked{seg, call, i, name, d.cwd}) {
					return
				}
			}
		}
	}
}

// state is one command's evaluation: the first ask reason wins, and the
// directory tracks cd segments so a later push or install checks the right
// repo.
type state struct {
	dirs
	// session is the hook payload's cwd, which a cd in the command doesn't move.
	session string
	h       *hook.Hook
	cfg     *config.Config
	pending string
	// managers is each repo root's verified manager facts, read once per command.
	managers map[string][]gapfill.Manager
}

func (st *state) ask(reason string) {
	if st.pending == "" {
		st.pending = reason
	}
}

// Check is the PreToolUse hook for Bash.
func Check(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	cmd := h.Payload.String(hook.KeyInputCommand)
	if cmd == "" {
		return nil
	}
	h.SetContext(layout.KV("cmd", layout.Clip(cmd)))
	cfg := cmp.Or(h.Config(), &config.Config{})
	home := h.Home
	norm := normalize(cmd)
	if err := wholeString(h, cfg, cmd, norm, home); err != nil {
		return err
	}
	session := resolveDir(home, "/", h.Payload.Cwd())
	st := &state{dirs: dirs{home: home, cwd: session}, session: session, h: h, cfg: cfg, managers: map[string][]gapfill.Manager{}}
	// The raw command, not norm: <<- strips tabs only, and norm would turn a
	// tab-indented terminator into spaces that no longer close the heredoc.
	segs, unparsed := parseAll(cmd)
	if unparsed {
		st.ask("guard-bash could not parse part of this command, so it went unchecked; confirm it")
	} else if onlyReads(segs) && !h.DryRun {
		if marker := cache.ReadOnlyBash(h.Paths.CacheDir(), h.Payload.SessionID(), cmd); marker != "" {
			_ = fsx.WriteAtomic(marker, nil, 0o600)
		}
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
	default:
		st.excludeSensitive(segs, cmd)
	}
	return nil
}

// excludeSensitive gives a lone rg with no glob of its own, in a work tree,
// the sensitive exclusion as its first flag, so a loose .env or *.pem
// under it isn't searched. The allow keeps auto mode from denying the
// rewrite, and approves only this read; the user's deny and ask rules still
// apply. A chained or piped rg keeps the shortcut at guard.SearchesSensitive.
func (st *state) excludeSensitive(segs []Segment, cmd string) {
	exclude := guard.SensitiveExclusion(st.cfg)
	rest, ok := strings.CutPrefix(strings.TrimSpace(cmd), "rg ")
	// onlyReads turns away a flag that runs a program (--pre, --hostname-bin),
	// which the allow would approve.
	if exclude == "" || !ok || len(segs) != 1 || len(segs[0].Calls) != 1 || segs[0].subshell || !onlyReads(segs) || !st.inWorktree() {
		return
	}
	for _, w := range segs[0].Calls[0].Words[1:] {
		if name, _, _ := strings.Cut(w.Value, "="); globFlags[name] != "" || strings.HasPrefix(name, "-g") {
			return
		}
	}
	rewritten := "rg --glob '" + exclude + "' " + rest
	input := maps.Clone(st.h.Payload.Object(hook.KeyToolInput))
	input["command"] = rewritten
	st.h.UpdateInput(input)
	st.h.Decide("allow", "kit excluded credential files from an rg inside the project")
	if marker := cache.ReadOnlyBash(st.h.Paths.CacheDir(), st.h.Payload.SessionID(), rewritten); marker != "" && !st.h.DryRun {
		_ = fsx.WriteAtomic(marker, nil, 0o600)
	}
}

// wholeString runs the checks on the whole command string, distinctive
// enough that quoted false positives aren't realistic.
func wholeString(h *hook.Hook, cfg *config.Config, cmd, norm, home string) error {
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
	if re := rcRedirect(cfg, home); re != nil && re.MatchString(normalize(withoutQuotedHeredocs(cmd))) {
		if err := h.Block(rcWriteReason, "rc-redirect"); err != nil {
			return err
		}
	}
	// On a quote-stripped copy: a && or | inside a quoted literal isn't one.
	if xargsRm.MatchString(dqQuoted.ReplaceAllString(sqQuoted.ReplaceAllString(cmd, ""), "")) {
		return h.Block("xargs rm with recursive or force flag", "xargs-rm")
	}
	return nil
}

// quotedHeredoc opens a heredoc whose quoted delimiter turns expansion off.
var quotedHeredoc = regexp.MustCompile(`<<-?[ \t]*['"]([A-Za-z_][A-Za-z0-9_]*)['"]`)

// withoutQuotedHeredocs drops quoted heredoc bodies, which run nothing as
// text; one fed to a shell is still checked, parsed as commands.
func withoutQuotedHeredocs(cmd string) string {
	var out, ends []string
	for line := range strings.Lines(cmd) {
		if len(ends) > 0 {
			if strings.TrimRight(strings.TrimLeft(line, "\t"), "\n") == ends[0] {
				ends = ends[1:]
				out = append(out, line)
			}
			continue
		}
		out = append(out, line)
		for _, m := range quotedHeredoc.FindAllStringSubmatch(line, -1) {
			ends = append(ends, m[1])
		}
	}
	return strings.Join(out, "")
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

// resolveDir resolves dir (relative, or under ~ or $HOME) against base,
// physically when it exists, so it compares equal to git's toplevel.
func resolveDir(home, base, dir string) string {
	dir = fsx.Abs(base, guard.ExpandHome(home, dir))
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		return real
	}
	return dir
}

// wrapperSpec is the grammar of a wrapper, a command that only changes how
// (or as whom) another runs: its options that take a separate value, and
// whether a word after them is its own operand rather than the command.
type wrapperSpec struct {
	values  []string
	operand func(word string) bool
}

// takes is true when option opt ("-u", "--user") takes a value.
func (s wrapperSpec) takes(opt string) bool { return slices.Contains(s.values, opt) }

func numeric(word string) bool { return word != "" && word[0] >= '0' && word[0] <= '9' }

func always(string) bool { return true }

// wrappers are the wrappers lead sees through. busybox and toybox run their
// first word as the applet; flock's first operand is its lock file.
var (
	wrappers = map[string]wrapperSpec{
		"command": {}, "builtin": {}, "nohup": {}, "stdbuf": {}, "unbuffer": {}, "busybox": {}, "toybox": {},
		"env":     {values: []string{"-u", "-C"}},
		"exec":    {values: []string{"-a"}},
		"nice":    {values: []string{"-n"}},
		"timeout": {values: []string{"-s", "-k"}, operand: numeric},
		"ionice":  {values: []string{"-c", "-n"}},
		"chrt":    {values: []string{"-p"}, operand: numeric},
		"sudo": {values: []string{"-u", "-g", "-h", "-p", "-C", "-D", "-R", "-T", "-U", "-r", "-t",
			"--user", "--group", "--host", "--prompt", "--close-from", "--chdir", "--chroot", "--command-timeout", "--other-user", "--role", "--type"}},
		"doas":       {values: []string{"-u", "-a", "-C"}},
		"caffeinate": {values: []string{"-t", "-w"}},
		"time":       {values: []string{"-f", "-o", "--format", "--output"}},
		"flock":      {values: []string{"-w", "-E", "--timeout", "--conflict-exit-code"}, operand: always},
		"watch":      {values: []string{"-n", "-q", "--interval", "--equexit"}},
	}
	assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
)

// wrapped is one wrapper in a call's words: its name, the index of its own
// word, and the index just past its options and operand.
type wrapped struct {
	name    string
	at, end int
}

// wrappedBy walks a call's leading VAR=value words and wrappers, each with
// its own options, and returns the wrappers and where the real command
// starts. Every reader of a wrapper's words starts from here.
func wrappedBy(words []Word) ([]wrapped, int) {
	var out []wrapped
	i := 0
	skipAssigns := func() {
		for i < len(words) && assignment.MatchString(words[i].Value) {
			i++
		}
	}
	skipAssigns()
	for i+1 < len(words) {
		name := baseName(words[i].Value)
		spec, ok := wrappers[name]
		if !ok {
			break
		}
		at := i
		i++
		for i < len(words)-1 && strings.HasPrefix(words[i].Value, "-") {
			opt := words[i].Value
			i++
			if opt == "--" {
				break
			}
			if takesNextWord(spec, opt) && i < len(words)-1 {
				i++
			}
		}
		if spec.operand != nil && i < len(words)-1 && spec.operand(words[i].Value) {
			i++
		}
		out = append(out, wrapped{name, at, i})
		skipAssigns()
	}
	return out, i
}

// lead is where the real command starts in a call's words.
func lead(words []Word) int {
	_, i := wrappedBy(words)
	return i
}

// takesNextWord is true when a wrapper's option consumes the next word. A
// short-option cluster (sudo -Eu) does when its first value-taking letter is
// its last; one earlier takes the rest of the word (sudo -uroot).
func takesNextWord(spec wrapperSpec, opt string) bool {
	if strings.HasPrefix(opt, "--") {
		// getopt_long takes any unambiguous prefix (sudo --us root); a
		// value after = is in the word already.
		if strings.Contains(opt, "=") {
			return false
		}
		return slices.ContainsFunc(spec.values, func(long string) bool { return strings.HasPrefix(long, "--") && abbrevOf(opt, long) })
	}
	for i := 1; i < len(opt); i++ {
		if spec.takes("-" + opt[i:i+1]) {
			return i == len(opt)-1
		}
	}
	return false
}

// interpreters run a script piped into them: every shell, and the script
// runtimes.
var interpreters = func() map[string]bool {
	m := maps.Clone(shells)
	for _, name := range []string{"fish", "su", "node", "ruby", "perl"} {
		m[name] = true
	}
	return m
}()

// pipeToShell blocks a download piped into an interpreter later in the
// same pipeline (curl x | sh, wget -O- x | tee log | sudo bash), and any
// other output piped into a shell reading stdin (echo ls | bash), a -c by
// another name. A heredoc cat passes on unchanged isn't: the parse checks
// it as commands, but only for shells, so any heredoc reaching fish is.
// Parsed calls, not the raw string: "curl x | sh" as quoted text is data.
func (st *state) pipeToShell(seg Segment) error {
	fetched, fed, heredoc := false, false, false
	for _, call := range seg.Calls {
		start := lead(call.Words)
		words := call.Words[start:]
		if len(words) == 0 {
			continue
		}
		name := baseName(words[0].Value)
		if fetched && (interpreters[name] || strings.HasPrefix(name, "python") || rootShell(call.Words[:start+1])) {
			return st.h.Block("piping network content into an interpreter", "pipe-to-shell")
		}
		heredoc = heredoc || len(call.Heredocs) > 0
		if runsC(name) && (fed || name == "fish" && heredoc) {
			shell := command{args: words[1:]}
			if wrap, script := shellMode(shell.values()); !wrap && !script {
				return st.h.Block(wrapReason, "interpreter-c-wrap")
			}
		}
		if name == "curl" || name == "wget" {
			fetched = true
		}
		fed = fed || !catsHeredoc(name, words[1:], call.Heredocs)
	}
	return nil
}

// catsHeredoc is true for a cat whose only input is its heredoc: no file
// operand (cat x <<EOF prints x), and no other command, which could rewrite
// the body (base64 -d <<EOF).
func catsHeredoc(name string, args []Word, heredocs []string) bool {
	if name != "cat" || len(heredocs) == 0 {
		return false
	}
	cat := command{args: args}
	return !slices.ContainsFunc(cat.operands(), func(op string) bool { return op != "-" })
}

// rootShell is true for sudo or doas told to start a shell (sudo -s, sudo -i,
// doas -s), which reads its commands from stdin like a named interpreter.
func rootShell(wrapperWords []Word) bool {
	elevator := ""
	for _, w := range wrapperWords {
		switch v := w.Value; {
		case baseName(v) == "sudo", baseName(v) == "doas":
			elevator = baseName(v)
		case elevator == "":
		case v == "--shell", v == "--login":
			return true
		case len(v) > 1 && v[0] == '-' && v[1] != '-':
			for i := 1; i < len(v); i++ {
				if v[i] == 's' || v[i] == 'i' {
					return true
				}
				if wrappers[elevator].takes("-" + v[i:i+1]) {
					break
				}
			}
		}
	}
	return false
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
		c := command{st: st, name: baseName(call.Words[start].Value), args: call.Words[start+1:], redirs: call.Redirs, inputs: call.Inputs}
		c.rest = seg.Rest(ci, start)
		c.text = c.name + c.rest
		c.alone = len(seg.Calls) == 1 && !seg.subshell
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
		if !strings.HasPrefix(r.Target.Raw, "'") && guard.IsRCFile(st.cfg, st.home, target) {
			if err := st.h.Block(rcWriteReason, "rc-redirect"); err != nil {
				return err
			}
		}
		st.askKitSwitch(target)
		st.looseWrite(">", target)
	}
	return nil
}

// looseWrite asks before a write (op is > or tee) that lands loose in the
// current directory, unless git tracks the target: that file is the
// project's already.
func (st *state) looseWrite(op, target string) {
	if (!strings.Contains(target, "/") || strings.HasPrefix(target, "./")) && looseWriteTarget(target) && st.inWorktree() && !st.tracked(target) {
		st.ask("'" + op + " " + target + "' writes into the current directory; skills/kit-rules/reference/tooling.md wants " + op + " \"$(kit scratch-dir)/" + baseName(target) + "\". Confirm only if this file belongs in the project tree.")
	}
}

// inWorktree is true when the tracked cwd (after any cd) is in a git work
// tree and not under a scratch directory: cd /tmp or cd .claude/scratch
// makes a bare > name harmless.
func (st *state) inWorktree() bool {
	if st.cwd == "" || project.IsScratch(st.h.Paths, st.cwd) {
		return false
	}
	return git.Toplevel(st.cwd) != ""
}

// tracked is true when git tracks target, a path relative to the tracked
// cwd; a failed lookup reads as untracked, so the ask stays.
func (st *state) tracked(target string) bool {
	paths, err := git.Paths(st.cwd, "ls-files", "-z", "--cached", "--", target)
	return err == nil && len(paths) > 0
}

// looseWriteTarget is true for a relative target that would land loose in
// the repo instead of scratch. Unresolvable targets (variables,
// substitutions, fd duplications) are false: $(kit scratch-dir) is the
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
// or inside a .claude/scratch directory. $(kit scratch-dir) counts; any
// other unexpanded variable, and any "..", doesn't, since it can't be
// checked.
func (st *state) scratchTarget(p string) bool {
	p = unquote(p)
	if strings.Contains(p, "..") {
		return false
	}
	switch {
	case p == "-", p == "/dev/null", len(p) == 2 && p[0] == '&' && p[1] >= '0' && p[1] <= '9',
		p == "$(kit scratch-dir)", strings.HasPrefix(p, "$(kit scratch-dir)/"),
		p == "`kit scratch-dir`", strings.HasPrefix(p, "`kit scratch-dir`/"):
		return true
	}
	p = st.abs(p)
	if strings.ContainsAny(p, "$`") {
		return false
	}
	return project.IsScratch(st.h.Paths, p)
}

// unquote strips one pair of surrounding quotes, single or double.
func unquote(p string) string {
	p = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(p, `"`), `'`), `"`)
	return strings.TrimSuffix(p, `'`)
}

// abs is p with ~ or $HOME expanded, joined to the segment's cwd when relative.
func (st *state) abs(p string) string {
	p = guard.ExpandHome(st.home, p)
	if !strings.HasPrefix(p, "/") {
		p = st.cwd + "/" + p
	}
	return p
}

// askKitSwitch asks, and is true, when a word (quoted, ~- or
// $HOME-prefixed, or relative to the segment's cwd) names a path
// guard.KitSwitch protects, through a symlink or not. Only a writer's
// operands and a redirect's target get here, so resolving each is cheap.
// The plugin-root exemption takes the session's cwd: a cd into the root
// within the command isn't work on the kit.
func (st *state) askKitSwitch(arg string) bool {
	what := st.kitSwitch(arg)
	if what != "" {
		st.ask(guard.KitSwitchAsk(what))
	}
	return what != ""
}

// kitSwitch is what guard.KitSwitch names for arg, resolved as
// askKitSwitch resolves it, or "".
func (st *state) kitSwitch(arg string) string {
	return guard.KitSwitch(st.h.Paths, st.session, st.abs(unquote(arg)))
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
	dir := st.cwd
	if gitDir != "" {
		dir = st.abs(gitDir)
	}
	branch, _ := git.Branch(dir)
	return branch
}

// readonlyCall is true for a lone `kit <read-only subcommand>` call
// (kitcmd.ReadOnly). Plugins can't ship allow rules, so the hook allows
// them itself; settings deny and ask rules still win over a hook allow. No
// chaining, pipes, redirects, or substitutions that could smuggle in a
// second command.
func readonlyCall(cmd, norm string) bool {
	if strings.ContainsAny(cmd, ";&|<>`\n") || strings.Contains(cmd, "$(") {
		return false
	}
	words := strings.Fields(norm)
	if len(words) < 2 || words[0] != "kit" || !kitcmd.ReadOnly(words[1]) {
		return false
	}
	return words[1] != "git-base" || gitBaseFlagsSafe(words[2:])
}

// gitBaseFlagsSafe is false when `kit git-base` would hand git a flag that can
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
