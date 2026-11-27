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
// moves as if it ran and worked, and prev, the cwd before the last cd, for
// cd -. maybe are the segments of cds that may not have; lost is true once
// a segment runs where one of them may not have, so the shell may be
// anywhere cwd passed through. A check that passes a path by where it
// lands reads cwd only while lost is false. lostIn is the subshell a cd
// moved, which the walk doesn't follow: the cwd is lost until it ends, then
// lostBefore again.
type dirs struct {
	home, cwd, prev string
	maybe           []int
	lost            bool
	lostIn          int
	lostBefore      bool
}

// enter readies d for segment j of segs: one past the subshell a cd moved
// finds the cwd again, and one that runs even when a cd in maybe didn't
// loses it.
func (d *dirs) enter(segs []Segment, j int) {
	if d.lostIn != 0 && !slices.Contains(segs[j].scopes, d.lostIn) {
		d.lost, d.lostIn = d.lostBefore, 0
	}
	if slices.ContainsFunc(d.maybe, func(k int) bool { return !andChain(segs, k, j) }) {
		d.maybe, d.lost = nil, true
	}
}

// loseIn loses the cwd for the rest of seg's innermost subshell, where a cd
// in seg moved the shell: (cd /; rm -rf x) runs rm in /. Outside any
// subshell, it's lost for good.
func (d *dirs) loseIn(seg Segment) {
	switch {
	case len(seg.scopes) == 0:
		d.maybe, d.lost = nil, true
	case d.lostIn == 0:
		d.lostIn, d.lostBefore, d.lost = seg.scopes[len(seg.scopes)-1], d.lost, true
	}
}

// movesShell is whether a cd at call ci of seg can move the shell that runs
// the rest of seg's subshell, or the command line: alone, or last in a
// pipeline, which zsh runs in the current shell (bash too, with lastpipe).
func movesShell(seg Segment, ci int) bool { return ci == len(seg.Calls)-1 }

// moveAt moves d as name, a cd, pushd, or popd standing alone in segment k
// of segs, with args, would. A cd surely moves the shell when it surely
// runs and set -e stops on its failure or its directory exists; one to an
// absolute directory then finds the cwd again. pushd, popd, and a cd the
// guard can't follow lose it.
func (d *dirs) moveAt(segs []Segment, k int, name string, args []Word) {
	if name != "cd" || !d.tracks(args) {
		d.maybe, d.lost = nil, true
		return
	}
	d.cd(args)
	seg := segs[k]
	switch {
	case seg.conditional || !fsx.IsDir(d.cwd) && (!standalone(seg) || !errexit(segs[:k])):
		d.maybe = append(d.maybe, k)
	case len(args) == 0 || filepath.IsAbs(guard.ExpandHome(d.home, args[0].Value)):
		d.maybe, d.lost = nil, false
	}
}

// movesDir is whether a call named name moves the shell's directory.
func movesDir(name string) bool { return name == "cd" || name == "pushd" || name == "popd" }

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
// index of its real command word, that command's base name, the directory
// it runs in, and whether it surely runs there (dirs.lost).
type walked struct {
	seg  Segment
	call Call
	i    int
	name string
	cwd  string
	sure bool
}

// walk yields each call of segs with a command word, run from cwd, a cd
// moving the directory for the calls after it.
func walk(segs []Segment, home, cwd string) iter.Seq[walked] {
	return walkPinned(segs, home, cwd, nil)
}

// pinFunc is words, in segment at, with each pinned variable read as its
// path (state.pinWords).
type pinFunc func(at int, words []Word) []Word

// walkPinned is walk with each call's words read through pin, when set.
func walkPinned(segs []Segment, home, cwd string, pin pinFunc) iter.Seq[walked] {
	return func(yield func(walked) bool) {
		d := dirs{home: home, cwd: cwd}
		for at, seg := range segs {
			d.enter(segs, at)
			for ci, call := range seg.Calls {
				if pin != nil {
					call.Words = pin(at, call.Words)
				}
				i := lead(call.Words)
				if i >= len(call.Words) {
					continue
				}
				name := baseName(call.Words[i].Value)
				// A cd before the last command of a pipeline moves nothing; one
				// that ends a pipeline, or sits in a subshell, moves a shell
				// the walk doesn't follow, so the cwd is lost.
				switch {
				case !movesDir(name) || !movesShell(seg, ci):
				case len(seg.Calls) == 1 && !seg.subshell:
					d.moveAt(segs, at, name, call.Words[i+1:])
				default:
					d.loseIn(seg)
				}
				if !yield(walked{seg, call, i, name, d.cwd, !d.lost}) {
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
	// pending is the command's first ask reason, and pendingRule its rule.
	pending, pendingRule string
	// shadowed is true once the command logged its kit-switch shadow.
	shadowed bool
	// segs is the whole command parsed, inner substitutions included.
	segs []Segment
	// managers is each repo root's verified manager facts, read once per command.
	managers map[string][]gapfill.Manager
	// vars is what the command's own bare assignments set (R=rm; $R -rf ~),
	// as of the segment being checked.
	vars map[string]string
	// loopVars is every value each loop's assignments set, by loop id.
	loopVars map[int]map[string][]string
	// sure is vars as the current shell surely has them: set by a lone
	// command outside any subshell and any part that may not run.
	sure map[string]string
	// sureAt is the segment each sure value was set in, and at the segment
	// being checked.
	sureAt map[string]int
	// chained is true for a sure value set on the right of && (track).
	chained map[string]bool
	at      int
	// exported is every name an export in the command names.
	exported map[string]bool
	// cmd is the whole command, as the Bash call gives it.
	cmd string
}

// begin readies st to check segs, cmd parsed, one segment at a time.
func (st *state) begin(cmd string, segs []Segment) {
	st.cmd = cmd
	st.vars, st.sure, st.sureAt, st.chained, st.loopVars, st.segs = map[string]string{}, map[string]string{}, map[string]int{}, map[string]bool{}, loopAssigned(segs), segs
	st.exported = exportedNames(segs)
}

// track records what seg, the i-th segment, assigns, once it's checked. An
// assignment on the right of && counts when every pipeline from its list's
// head to it follows an && (ls && S=x): a use later in that chain runs only
// if it ran (sureValue). One after || or in a compound doesn't.
func (st *state) track(i int, seg Segment) {
	vars := assigned(seg)
	maps.Copy(st.vars, vars)
	chained := seg.conditional && st.chainedFromHead(i)
	for name, value := range vars {
		switch {
		case seg.subshell || len(seg.Calls) > 1 || seg.conditional && !chained:
			delete(st.sure, name)
		default:
			st.sure[name], st.sureAt[name], st.chained[name] = value, i, chained
		}
	}
}

// chainedFromHead is true when segment i runs only if every pipeline
// before it in its list succeeded: an && joins each pair from the nearest
// earlier pipeline that always runs.
func (st *state) chainedFromHead(i int) bool {
	for head := i - 1; head >= 0; head-- {
		if seg := st.segs[head]; !seg.subshell && !seg.conditional {
			return andChain(st.segs, head, i)
		}
	}
	return false
}

// sureValue is name's sure value at segment use: one set on the right of
// && counts only where use runs in the same && chain after it.
func (st *state) sureValue(name string, use int) (string, bool) {
	value, ok := st.sure[name]
	if ok && st.chained[name] && !andChain(st.segs, st.sureAt[name], use) {
		return "", false
	}
	return value, ok
}

// setVars is vars as NAME=value words, in no set order, split into those
// the command exports and the bare rest, which reach a child process only
// if the environment already exported them.
func (st *state) setVars() (exported, bare []string) {
	for name, value := range st.vars {
		if st.exported[name] {
			exported = append(exported, name+"="+value)
		} else {
			bare = append(bare, name+"="+value)
		}
	}
	return exported, bare
}

// pinRefs is each $NAME or ${NAME} in a string.
var pinRefs = regexp.MustCompile(nameRef)

// pinBase is a value that starts with one $NAME or ${NAME} and has plain
// text after it: its groups are the name (twice) and the text.
var pinBase = regexp.MustCompile(`^` + nameRef + "([^$`]*)$")

// pinned is the plain path name surely holds at the segment being checked:
// set once, outside any loop, to a plainWord (nothing the shell splits,
// globs, or expands, ~ included), or to an earlier pinned variable with
// plain text after it, up to depth levels. Nothing is pinned that could be
// set unseen (setOnly), that a for or select sets, or when the command
// names IFS, which would split it.
func (st *state) pinned(name string, depth int) (string, bool) {
	value, ok := st.sureValue(name, st.at)
	if !ok || st.inLoop(name) || !st.setOnly(name) {
		return "", false
	}
	cmd := st.cmd
	q := regexp.QuoteMeta(name)
	if regexp.MustCompile(`\bIFS\b|\b(?:for|select)\s+` + q + `\b`).MatchString(cmd) {
		return "", false
	}
	if m := pinBase.FindStringSubmatch(value); m != nil {
		inner := m[1] + m[2]
		if depth == 0 || !st.setFromEarlier(name, inner) {
			return "", false
		}
		base, ok := st.pinned(inner, depth-1)
		if !ok {
			return "", false
		}
		value = base + m[3]
	}
	return value, plainWord.MatchString(value)
}

// pinnedHome is the HOME the command pinned to an absolute path, else the
// session's.
func (st *state) pinnedHome() string {
	if home, ok := st.pinned("HOME", 2); ok && strings.HasPrefix(home, "/") {
		return home
	}
	return st.home
}

// setFromEarlier is true when name's sure value was set from $inner, which
// the command surely set before it.
func (st *state) setFromEarlier(name, inner string) bool {
	at, set := st.sureAt[inner]
	return set && at < st.sureAt[name] && setFrom(st.cmd, name, inner)
}

// setFrom is true when cmd sets name to $inner, bare or in double quotes:
// '$S' and \$S set the text $S.
func setFrom(cmd, name, inner string) bool {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `="?\$\{?` + regexp.QuoteMeta(inner) + `\b`).MatchString(cmd)
}

// inLoop is true when a loop in the command assigns name.
func (st *state) inLoop(name string) bool {
	return slices.ContainsFunc(slices.Collect(maps.Values(st.loopVars)), func(loop map[string][]string) bool {
		_, ok := loop[name]
		return ok
	})
}

// expandPins is s with each reference to a pinned variable replaced by its
// path; any other reference stays.
func (st *state) expandPins(s string) string {
	if !strings.Contains(s, "$") {
		return s
	}
	return pinRefs.ReplaceAllStringFunc(s, func(ref string) string {
		m := pinRefs.FindStringSubmatch(ref)
		if path, ok := st.pinned(m[1]+m[2], 2); ok {
			return path
		}
		return ref
	})
}

// pinWords is words with expandPins applied to each that has no single
// quote or backslash, which would keep a $ as text.
func (st *state) pinWords(words []Word) []Word {
	out := slices.Clone(words)
	for i, w := range out {
		if !strings.ContainsAny(w.Raw, `'\`) {
			out[i].Value = st.expandPins(w.Value)
		}
	}
	return out
}

func (st *state) ask(reason, rule string) {
	if st.pending == "" {
		st.pending, st.pendingRule = reason, rule
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
	segs, unparsed, unplaced := parseFull(cmd)
	switch {
	case unparsed:
		st.ask("guard-bash could not parse part of this command, so it went unchecked; confirm it", "unparsed")
	case unplaced:
		st.ask("guard-bash can't place this trap's action in the command (an expansion, a signal other than EXIT, or a nested scope), so where it runs went unchecked; confirm it", "trap-unplaced")
	}
	st.begin(cmd, segs)
	for i, seg := range segs {
		st.at = i
		st.enter(segs, i)
		if err := st.segment(seg); err != nil {
			return err
		}
		st.track(i, seg)
	}
	runs := cmd
	switch {
	case st.pending != "":
		h.Ask(st.pending, st.pendingRule)
	case readonlyCall(cmd, norm):
		h.Allow("side-effect-free claude-kit script")
	default:
		if rewritten := st.excludeSensitive(segs, cmd); rewritten != "" {
			runs = rewritten
		}
	}
	// bash-edits reads the marker of the command that runs, the rewrite
	// when there is one. A rewrite only reads, rg piped into sort included.
	if !unparsed && !unplaced && (onlyReads(segs) || runs != cmd) && !h.DryRun {
		if marker := cache.ReadOnlyBash(h.Paths.CacheDir(), h.Payload.SessionID(), runs); marker != "" {
			_ = fsx.WriteAtomic(marker, nil, 0o600)
		}
	}
	return nil
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
		if fed && startsRootShell(call.Words) {
			return st.h.Block(wrapReason, "interpreter-c-wrap")
		}
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

// segment runs the per-command checks on every command of a pipeline.
func (st *state) segment(seg Segment) error {
	if err := st.pipeToShell(seg); err != nil {
		return err
	}
	st.dirToNetwork(seg)
	for ci, call := range seg.Calls {
		if err := st.redirects(call); err != nil {
			return err
		}
		if runsString(call.Words) {
			return st.h.Block(wrapReason, "interpreter-c-wrap")
		}
		start := lead(call.Words)
		if start >= len(call.Words) {
			// A redirect with no command still reads its input: x=$(<.env).
			if err := (&command{st: st}).readsSensitive(call.Inputs); err != nil {
				return err
			}
			continue
		}
		// $(echo rm) -rf ~ names its command only at runtime, as does
		// sh <<< "$(echo rm -rf ~)" once its body is parsed.
		if raw := call.Words[start].Raw; strings.Contains(raw, "$(") || strings.Contains(raw, "`") {
			st.ask("the command name comes from a substitution, so guard-bash can't check it; confirm it", "computed-command")
		}
		// One check per value $R may hold; a plain name is checked once.
		expansions := st.varFields(seg, call.Words[start])
		if len(expansions) == 0 {
			expansions = [][]string{nil}
		}
		plain := command{
			st: st, name: baseName(call.Words[start].Value), args: call.Words[start+1:], redirs: call.Redirs, inputs: call.Inputs,
			rest: seg.Rest(ci, start), alone: len(seg.Calls) == 1 && !seg.subshell, shellString: call.shellString, movesShell: movesShell(seg, ci),
			assigns: append(slices.Clone(call.Assigns), slices.DeleteFunc(values(call.Words[:start]), func(w string) bool { return !assignment.MatchString(w) })...),
		}
		for _, fields := range expansions {
			c := plain
			if len(fields) > 0 {
				// The shell splits an unquoted $R into the name and leading args.
				lifted := make([]Word, 0, len(fields)-1+len(c.args))
				for _, f := range fields[1:] {
					lifted = append(lifted, Word{Value: f, Raw: f, literal: true})
				}
				if len(lifted) > 0 {
					c.rest = " " + strings.Join(fields[1:], " ") + c.rest
				}
				// The parser never saw this name, so it read no -c string.
				c.name, c.args, c.shellString = baseName(fields[0]), append(lifted, c.args...), false
			}
			c.text = c.name + c.rest
			if err := c.check(); err != nil {
				return err
			}
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
		// &1 or &- duplicates or closes an fd; it names no file.
		if !strings.HasPrefix(target, "&") {
			st.askKitSwitch(target)
		}
		st.looseWrite(">", target)
	}
	return nil
}

// looseWrite asks before a write (op is > or tee) that lands loose in the
// current directory, unless git tracks the target: that file is the
// project's already. Where the shell may be elsewhere, it asks.
func (st *state) looseWrite(op, target string) {
	if (!strings.Contains(target, "/") || strings.HasPrefix(target, "./")) && looseWriteTarget(target) && (st.lost || st.inWorktree() && !st.tracked(target)) {
		st.ask("'"+op+" "+target+"' writes into the current directory; skills/kit-rules/reference/tooling.md wants "+op+" \"$(kit scratch-dir)/"+baseName(target)+"\". Confirm only if this file belongs in the project tree.", "loose-write")
	}
}

// inWorktree is true when the tracked cwd (after any cd) is in a git work
// tree and not under a scratch directory: cd /tmp or cd .claude/scratch
// makes a bare > name harmless.
func (st *state) inWorktree() bool { return st.worktreeTop() != "" }

// worktreeTop is the toplevel of the work tree cwd is in, "" outside one
// or in scratch.
func (st *state) worktreeTop() string {
	if st.cwd == "" || st.h.Paths.IsScratch(st.cwd) {
		return ""
	}
	return git.Toplevel(st.cwd)
}

// tracked is true when git tracks target, a path relative to the tracked
// cwd; a failed lookup reads as untracked, so the ask stays.
func (st *state) tracked(target string) bool {
	// literal: a glob in target ('main*') would match other tracked files.
	paths, err := git.Paths(st.cwd, "ls-files", "-z", "--cached", "--", ":(literal)"+target)
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
// or inside a .claude/scratch directory. $(kit scratch-dir), the kit's own
// bin/kit by any path included, and $PWD count; any other unexpanded
// variable, and any "..", doesn't, since it can't be checked. A path whose
// symlinks lead out of scratch doesn't either; followLast says whether a
// final link counts, as it does for a write, and not for rm, which removes
// the link itself.
func (st *state) scratchTarget(p string, followLast bool) bool {
	p = unquote(p)
	switch {
	case p == "-", p == "/dev/null", len(p) == 2 && p[0] == '&' && p[1] >= '0' && p[1] <= '9':
		return true
	}
	path, ok := st.scratchPath(p)
	return ok && st.onDisk(path, followLast)
}

// scratchPath is p, unquoted, as an absolute path, with $(kit scratch-dir)
// read as the directory it prints and $PWD, $(pwd), ~, and $HOME expanded. False
// with a "..", or any other expansion left in it.
func (st *state) scratchPath(p string) (string, bool) {
	if strings.Contains(p, "..") {
		return "", false
	}
	if runsIn, rest, ok := st.kitScratchDir(p); ok {
		dir, err := project.ScratchDir(st.h.Paths, runsIn, false)
		return dir + rest, err == nil && !strings.ContainsAny(rest, "$`")
	}
	for _, pwd := range []string{"$PWD", "${PWD}", "$(pwd)", "`pwd`"} {
		if st.cwd != "" && (p == pwd || strings.HasPrefix(p, pwd+"/")) {
			if st.lost {
				return "", false
			}
			p = st.cwd + p[len(pwd):]
			break
		}
	}
	ok := st.placed(p)
	p = st.abs(p)
	return p, ok && !strings.ContainsAny(p, "$`")
}

// onDisk is true when p, absolute, is in scratch as written and on disk:
// resolving its symlinks keeps it under its scratch root, resolved too. A
// trailing slash follows a final link, as rm -r link/ does.
func (st *state) onDisk(p string, followLast bool) bool {
	root, ok := st.h.Paths.ScratchRoot(p)
	if !ok {
		return false
	}
	if root == filepath.Clean(p) && !followLast {
		return true
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		// No root, so nothing under it to follow.
		return true
	}
	if followLast || strings.HasSuffix(p, "/") {
		p = fsx.PhysicalPath(filepath.Clean(p))
	}
	return fsx.Within(real, fsx.ResolvedParent(filepath.Clean(p)))
}

// kitScratchSub matches a path that starts with a kit scratch-dir call,
// $(K scratch-dir) or `K scratch-dir`, past a leading `cd DIR &&` and a
// KIT_DEV=1, which picks how bin/kit builds, not what it prints. Its groups
// are DIR and K, once per form.
var kitScratchSub = regexp.MustCompile("^(?:\\$\\((?:cd ([^\\s$`()&;|]+) && )?(?:KIT_DEV=1 )?([^\\s$`()=]+) scratch-dir\\)" +
	"|`(?:cd ([^\\s$`()&;|]+) && )?(?:KIT_DEV=1 )?([^\\s$`()=]+) scratch-dir`)(?:/|$)")

// kitScratchDir reads p's leading scratch-dir call to kit on PATH, or to
// this kit's bin/kit by any path: the directory the call runs in, past its
// cd, and the rest of p. ok is false for any other call, since a kit
// anywhere else could print any directory.
func (st *state) kitScratchDir(p string) (runsIn, rest string, ok bool) {
	m := kitScratchSub.FindStringSubmatch(p)
	if m == nil {
		return "", "", false
	}
	runsIn, kit, cd := st.cwd, m[2]+m[4], m[1]+m[3]
	if cd != "" {
		if !st.placed(cd) {
			return "", "", false
		}
		runsIn = resolveDir(st.pinnedHome(), st.cwd, st.expandPins(cd))
	}
	rest = p[len(strings.TrimSuffix(m[0], "/")):]
	if kit == "kit" {
		return runsIn, rest, true
	}
	path := guard.ExpandHome(st.pinnedHome(), st.expandPins(kit))
	if !filepath.IsAbs(path) {
		if cd == "" && !st.placed(kit) {
			return "", "", false
		}
		path = filepath.Join(runsIn, path)
	}
	return runsIn, rest, st.h.Paths.Root != "" && filepath.Clean(path) == filepath.Join(st.h.Paths.Root, "bin", "kit")
}

// unquote strips one pair of surrounding quotes, single or double.
func unquote(p string) string {
	p = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(p, `"`), `'`), `"`)
	return strings.TrimSuffix(p, `'`)
}

// abs is p with its pinned variables, ~, and $HOME expanded, ~ by a HOME
// the command pinned to an absolute path, joined to the segment's cwd when
// relative.
func (st *state) abs(p string) string {
	p = guard.ExpandHome(st.pinnedHome(), st.expandPins(p))
	if !strings.HasPrefix(p, "/") {
		p = st.cwd + "/" + p
	}
	return p
}

// placed is true when p, as abs reads it, lands where abs puts it: it's
// absolute, or the shell surely runs in the tracked cwd.
func (st *state) placed(p string) bool {
	return !st.lost || strings.HasPrefix(guard.ExpandHome(st.pinnedHome(), st.expandPins(p)), "/")
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
		st.ask(guard.KitSwitchAsk(what), "kit-switch")
	}
	return what != ""
}

// kitSwitch is what guard.KitSwitch names for arg, resolved as
// askKitSwitch resolves it, or "".
func (st *state) kitSwitch(arg string) string {
	return guard.KitSwitch(st.h.Paths, st.session, st.abs(unquote(arg)))
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

// statWidths is --stat with its widths and count, which only shape the output.
var statWidths = regexp.MustCompile(`^--stat=\d+(,\d+){0,2}$`)

// gitBaseFlagsSafe is false when `kit git-base` would hand git a flag that can
// write files (--output) or run programs (--ext-diff): only the flags the
// kit's own skills pass, and help, go through without a prompt. Words after
// -- are pathspecs git gets after its own -- (gitbase.Run).
func gitBaseFlagsSafe(words []string) bool {
	// The shell drops quotes, backslashes, and $'': '--output=x' and
	// \--output=x reach git as --output=x.
	unquote := strings.NewReplacer(`'`, "", `"`, "", `\`, "", "$", "")
	for _, w := range words {
		w = unquote.Replace(w)
		switch {
		case w == "--":
			return true
		case !strings.HasPrefix(w, "-"):
		case len(w) > 1 && w[1] >= '0' && w[1] <= '9':
		case slices.Contains([]string{"--diff", "--log", "--stat", "--name-only", "--name-status", "--no-merges", "--oneline", "--shortstat", "--help", "-h"}, w),
			statWidths.MatchString(w):
		default:
			return false
		}
	}
	return true
}
