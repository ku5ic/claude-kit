package bashguard

import (
	"cmp"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitcmd"
	"github.com/ku5ic/claude-kit/go/internal/layout"
)

// state is one command's evaluation: the first ask reason wins, and the
// directory tracks cd segments so a later push or install checks the right
// repo.
type state struct {
	dirs
	shellVars
	h   *hook.Hook
	cfg *config.Config
	// pending is the command's first ask reason, and pendingRule its rule.
	pending, pendingRule string
	// shadowed is true once the command logged its kit-switch shadow.
	shadowed bool
	// managers is each repo root's verified manager facts, read once per command.
	managers map[string][]gapfill.Manager
}

// pinnedHome is the HOME the command pinned to an absolute path, else the
// session's.
func (st *state) pinnedHome() string {
	if home, ok := st.pinned("HOME", 2); ok && strings.HasPrefix(home, "/") {
		return home
	}
	return st.home
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
	st := &state{dirs: dirs{home: home, cwd: session}, h: h, cfg: cfg, managers: map[string][]gapfill.Manager{}}
	// The raw command, not norm: <<- strips tabs only, and norm would turn a
	// tab-indented terminator into spaces that no longer close the heredoc.
	p := parseFull(cmd, h.Shell)
	st.askUnchecked(p)
	st.begin(cmd, p.segs)
	for i, seg := range p.segs {
		st.at = i
		st.enter(p.segs, i)
		if err := st.segment(seg); err != nil {
			return err
		}
		st.track(i, seg)
	}
	st.decide(p, cmd, norm)
	return nil
}

// askUnchecked asks when part of the command went unchecked: a line that
// didn't parse, a trap the guard can't place, or a zsh expansion that runs
// code.
func (st *state) askUnchecked(p parsed) {
	switch {
	case p.unparsed:
		st.ask("guard-bash could not parse part of this command, so it went unchecked; confirm it", "unparsed")
	case p.unplaced:
		st.ask("guard-bash can't place this trap's action in the command (an expansion, a signal like DEBUG or ERR, a directory change with a mid-command signal, or a nested scope), so where it runs went unchecked; confirm it", "trap-unplaced")
	case p.runsCode:
		st.ask("a zsh glob qualifier or expansion flag here can run a command as it expands (e:, +, ${(e)...}, ${~...}), which guard-bash can't check; confirm it", "zsh-expansion")
	}
}

// decide gives the command's verdict once every segment passed: the first
// ask, an allow for a kit script that only reads, or the rewrite that
// excludes credential files. bash-edits reads the marker of the command
// that runs, the rewrite when there is one; a rewrite only reads, rg piped
// into sort included.
func (st *state) decide(p parsed, cmd, norm string) {
	h, runs := st.h, cmd
	switch {
	case st.pending != "":
		h.Ask(st.pending, st.pendingRule)
	case readonlyCall(cmd, norm):
		h.Allow("side-effect-free claude-kit script")
	default:
		if rewritten := st.excludeSensitive(p.segs, cmd); rewritten != "" {
			runs = rewritten
		}
	}
	if p.unparsed || p.unplaced || p.runsCode || h.DryRun || !onlyReads(p.segs) && runs == cmd {
		return
	}
	if marker := cache.ReadOnlyBash(h.Paths.CacheDir(), h.Payload.SessionID(), runs); marker != "" {
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
		heredoc = heredoc || len(call.Heredocs) > 0
		if err := st.pipedInto(call, start, fetched, fed, heredoc); err != nil {
			return err
		}
		fetched = fetched || name == "curl" || name == "wget"
		fed = fed || !catsHeredoc(name, words[1:], call.Heredocs)
	}
	return nil
}

// pipedInto blocks call, its command at start, when what the pipeline
// before it feeds it runs as code: fetched content into an interpreter,
// any output into a root shell or a shell reading stdin, or a heredoc into
// fish.
func (st *state) pipedInto(call Call, start int, fetched, fed, heredoc bool) error {
	words := call.Words[start:]
	name := baseName(words[0].Value)
	if fetched && (interpreters[name] || strings.HasPrefix(name, "python") || rootShell(call.Words[:start+1])) {
		return st.h.Block("piping network content into an interpreter", "pipe-to-shell")
	}
	if fed && startsRootShell(call.Words) {
		return st.h.Block(wrapReason, "interpreter-c-wrap")
	}
	if !runsC(name) || !fed && (name != "fish" || !heredoc) {
		return nil
	}
	shell := command{args: words[1:]}
	if wrap, script := shellMode(shell.values()); !wrap && !script {
		return st.h.Block(wrapReason, "interpreter-c-wrap")
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
		if err := st.call(seg, ci, call); err != nil {
			return err
		}
	}
	return nil
}

// call checks one command of seg, the ci-th, once per value its name may
// expand to; a plain name is checked once.
func (st *state) call(seg Segment, ci int, call Call) error {
	start := lead(call.Words)
	if start >= len(call.Words) {
		// A redirect with no command still reads its input: x=$(<.env).
		return (&command{st: st}).readsSensitive(call.Inputs)
	}
	// $(echo rm) -rf ~ names its command only at runtime, as does
	// sh <<< "$(echo rm -rf ~)" once its body is parsed.
	if raw := call.Words[start].Raw; strings.Contains(raw, "$(") || strings.Contains(raw, "`") {
		st.ask("the command name comes from a substitution, so guard-bash can't check it; confirm it", "computed-command")
	}
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
		c := plain.as(fields)
		if err := c.check(); err != nil {
			return err
		}
	}
	return nil
}

// as is c with its name expanded to fields, one value of a variable: the
// shell splits an unquoted $R into the name and leading args. The parser
// never saw that name, so it read no -c string. With no fields c keeps its
// name.
func (c command) as(fields []string) command {
	if len(fields) > 0 {
		lifted := make([]Word, 0, len(fields)-1+len(c.args))
		for _, f := range fields[1:] {
			lifted = append(lifted, Word{Value: f, Raw: f, literal: true})
		}
		if len(lifted) > 0 {
			c.rest = " " + strings.Join(fields[1:], " ") + c.rest
		}
		c.name, c.args, c.shellString = baseName(fields[0]), append(lifted, c.args...), false
	}
	c.text = c.name + c.rest
	return c
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
