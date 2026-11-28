package bashguard

import (
	"github.com/ku5ic/claude-kit/go/internal/classify"
	"regexp"
	"slices"
	"strings"
)

// shellCString returns the string a POSIX shell's -c runs, past any
// wrapper: bash -c 'echo hi' runs echo hi. Only a literal shell name and a
// literal string that ends the call count, so what the parser reads is
// what the shell runs: words after it are its $0 and $1, which the string
// alone doesn't show. fish's strings aren't POSIX, and stay blocked.
func shellCString(words []Word) (string, bool) {
	i := lead(words)
	if i >= len(words) || !words[i].literal || !shells[baseName(words[i].Value)] {
		return "", false
	}
	args := words[i+1:]
	if wrap, _ := shellMode(values(args)); !wrap {
		return "", false
	}
	at := cString(values(args))
	if at < 0 || at != len(args)-1 || !args[at].literal {
		return "", false
	}
	return args[at].Value, true
}

// rebaseExecs is each command string a git rebase among words runs after
// each commit: -x cmd, -xcmd, either in a cluster such as -ix, --exec cmd,
// or --exec=cmd.
func rebaseExecs(words []Word) []string {
	i := lead(words)
	if i >= len(words) || baseName(words[i].Value) != "git" {
		return nil
	}
	sub, args, _ := classify.GitSubcommand(values(words[i+1:]))
	if sub != "rebase" {
		return nil
	}
	return execArgs(args)
}

// execArgs is each command string git rebase's args run after each commit.
// git takes any unambiguous prefix of a long option: --ex is --exec.
func execArgs(args []string) []string {
	isExec := func(flag string) bool { return len(flag) >= len("--ex") && abbrevOf(flag, "--exec") }
	var out []string
	for j := 0; j < len(args); j++ {
		a := args[j]
		flag, value, attached := strings.Cut(a, "=")
		switch {
		case attached && isExec(flag):
			out = append(out, value)
		case isExec(a) && j+1 < len(args):
			out = append(out, args[j+1])
			j++
		case len(a) > 1 && a[0] == '-' && a[1] != '-':
			cmd, ok := clusterExec(a[1:])
			if ok && cmd == "" && j+1 < len(args) {
				cmd = args[j+1]
				j++
			}
			if cmd != "" {
				out = append(out, cmd)
			}
		}
	}
	return out
}

// clusterExec is the exec command in letters, a short-option cluster of
// git rebase such as ix or xcmd: the rest after its x, "" when the x ends
// it and the command is the next word. A letter that takes a value (-s,
// -X, -C) holds the rest of the cluster, so an x after it is data.
func clusterExec(letters string) (string, bool) {
	for i, c := range letters {
		switch c {
		case 'x':
			return letters[i+1:], true
		case 's', 'X', 'C':
			return "", false
		}
	}
	return "", false
}

// fdTarget is a >& target that names a descriptor: 2, 2-, or -.
var fdTarget = regexp.MustCompile(`^([0-9]+-?|-)$`)

// flockString is the command string a flock among words' wrappers runs
// with -c or --command, before its lock file or right after it, since flock
// permutes its options: flock -c "rm -rf ~" /tmp/l and flock /tmp/l -c
// "rm -rf ~" both run rm -rf ~ in a shell.
func flockString(words []Word) (string, bool) {
	runs, n := wrappedBy(words)
	for _, w := range runs {
		if w.name != "flock" {
			continue
		}
		for k := w.at + 1; k < w.end && words[k].Value != "--"; k++ {
			if cmd, ok := flockCommand(words, k); ok {
				return cmd, true
			}
		}
		if w.end == n && n < len(words) {
			return flockCommand(words, n)
		}
	}
	return "", false
}

// flockCommand reads words[k] as flock's -c or --command, in a short
// cluster too (-nc), and returns the string it runs: the rest of the word
// or the next one.
func flockCommand(words []Word, k int) (string, bool) {
	opt := words[k].Value
	next := func() (string, bool) {
		if k+1 < len(words) {
			return words[k+1].Value, true
		}
		return "", false
	}
	if name, value, eq := strings.Cut(opt, "="); strings.HasPrefix(opt, "--") {
		if !abbrevOf(name, "--command") {
			return "", false
		}
		if eq {
			return value, true
		}
		return next()
	}
	if !strings.HasPrefix(opt, "-") {
		return "", false
	}
	for i := 1; i < len(opt); i++ {
		switch {
		case opt[i] == 'c' && i+1 < len(opt):
			return opt[i+1:], true
		case opt[i] == 'c':
			return next()
		case wrappers["flock"].takes("-" + opt[i:i+1]):
			return "", false
		}
	}
	return "", false
}

// watchString is the command a watch among words' wrappers runs: its
// operands joined, which watch hands to sh -c unless -x execs them.
func watchString(words []Word) (string, bool) {
	runs, _ := wrappedBy(words)
	for _, w := range runs {
		if w.name != "watch" {
			continue
		}
		i := w.at
		j := i + 1
		for ; j < len(words) && strings.HasPrefix(words[j].Value, "-"); j++ {
			opt := words[j].Value
			if opt == "--" {
				j++
				break
			}
			if abbrevOf(opt, "--exec") || shortCluster(opt, "x") {
				return "", false
			}
			if takesNextWord(wrappers["watch"], opt) {
				j++
			}
		}
		if j >= len(words) {
			return "", false
		}
		var parts []string
		for _, w := range words[j:] {
			parts = append(parts, w.Value)
		}
		return strings.Join(parts, " "), true
	}
	return "", false
}

// timeOutputs are the files a time among words' wrappers writes its report
// to with -o, as > redirects, so they get a redirect's checks.
func timeOutputs(words []Word) []Redir {
	var out []Redir
	runs, _ := wrappedBy(words)
	for _, w := range runs {
		if w.name != "time" {
			continue
		}
		found, done := timeOptions(words, w.at)
		out = append(out, found...)
		if done {
			return out
		}
	}
	return out
}

// timeOptions reads the options of the time at words[at]. done is true at
// --, which ends the scan for every wrapper after it too.
func timeOptions(words []Word, at int) (out []Redir, done bool) {
	for j := at + 1; j < len(words) && strings.HasPrefix(words[j].Value, "-"); j++ {
		opt := words[j].Value
		switch {
		case opt == "--":
			return out, true
		case abbrevOf(opt, "--output"):
			// getopt_long takes any unambiguous prefix: --out file.
			target, at, ok := longValue(words, j)
			if ok {
				out = append(out, Redir{Op: ">", Target: target})
			}
			j = at
		case strings.HasPrefix(opt, "--"):
			if takesNextWord(wrappers["time"], opt) {
				j++
			}
		default:
			// A short cluster: its first f or o takes the rest of the
			// word, or the next word when it's last (-ao file).
			k := strings.IndexAny(opt[1:], "fo") + 1
			if k == 0 {
				continue
			}
			v := opt[k+1:]
			if v == "" && j+1 < len(words) {
				j++
				v = words[j].Value
			}
			if opt[k] == 'o' && v != "" {
				out = append(out, Redir{Op: ">", Target: Word{Value: v, Raw: v}})
			}
		}
	}
	return out, false
}

// longValue is the value of the long option at words[j], after its = or
// in the next word, and the last word it takes; ok is false when it has
// none.
func longValue(words []Word, j int) (value Word, at int, ok bool) {
	if _, v, eq := strings.Cut(words[j].Value, "="); eq {
		return Word{Value: v, Raw: v}, j, true
	}
	if j+1 < len(words) {
		return words[j+1], j + 1, true
	}
	return Word{}, j, false
}

// trapAction is the command trap [--] action signal... sets. trap -p and
// -l only print, and one word or a - action resets a trap. readable is
// false for an action with an expansion in it, or one a signal outside
// atExit and termination fires, since the guard can't place it. midway is
// whether a termination signal fires it.
func trapAction(words []Word) (action string, readable, midway, ok bool) {
	i := lead(words)
	if i >= len(words) || baseName(words[i].Value) != "trap" {
		return "", false, false, false
	}
	args := words[i+1:]
	switch {
	case len(args) > 0 && args[0].Value == "--":
		args = args[1:]
	case len(args) > 0 && strings.HasPrefix(args[0].Value, "-") && args[0].Value != "-":
		return "", false, false, false
	}
	if len(args) < 2 || args[0].Value == "-" {
		return "", false, false, false
	}
	readable = args[0].literal && !slices.ContainsFunc(args[1:], func(w Word) bool { return !atExit[w.Value] && !termination[w.Value] })
	midway = slices.ContainsFunc(args[1:], func(w Word) bool { return termination[w.Value] })
	return args[0].Value, readable, midway, true
}

// atExit fire a trap when the shell exits; termination fire it on a signal
// that ends the command by default, mid-command. DEBUG, ERR, and RETURN
// fire it again and again, so they're in neither.
var (
	atExit      = nameSet("EXIT", "0")
	termination = nameSet("HUP", "INT", "QUIT", "TERM", "SIGHUP", "SIGINT", "SIGQUIT", "SIGTERM")
)

// envSplitString returns the command env -S/--split-string runs, the string
// followed by env's remaining words, from a wrapper before the command:
// env -S "rm -rf" ~ runs rm -rf ~.
func envSplitString(words []Word) (string, bool) {
	runs, _ := wrappedBy(words)
	for _, w := range runs {
		if w.name != "env" {
			continue
		}
		for j := w.at + 1; j < len(words) && strings.HasPrefix(words[j].Value, "-"); j++ {
			cmd, at, verdict := envOption(words, j)
			switch verdict {
			case envStop:
				return "", false
			case envFound:
				for _, w := range words[at+1:] {
					cmd += " " + w.Raw
				}
				return cmd, true
			}
			j = at
		}
	}
	return "", false
}

// envVerdict is what one of env's options means for the -S scan.
type envVerdict int

const (
	envNext  envVerdict = iota // scan on after the word at
	envStop                    // no -S string: --, or -S with nothing after it
	envFound                   // the -S string, ending at word at
)

// envOption reads env's option at words[j]: the -S string it gives, the
// last word it takes, and what it means for the scan.
func envOption(words []Word, j int) (cmd string, at int, verdict envVerdict) {
	opt := words[j].Value
	name, value, eq := strings.Cut(opt, "=")
	switch {
	case opt == "--":
		return "", j, envStop
	case abbrevOf(name, "--split-string") && eq:
		return value, j, envFound
	case abbrevOf(name, "--split-string") && j+1 >= len(words):
		return "", j, envStop
	case abbrevOf(name, "--split-string"):
		return words[j+1].Value, j + 1, envFound
	case strings.HasPrefix(opt, "--"):
		return "", j + boolInt(takesNextWord(wrappers["env"], opt)), envNext
	}
	return envCluster(words, j)
}

// envCluster reads a short cluster (-iS, -C/tmp) as envOption does: its
// first S, u, or C takes the rest of the word, or the next word when it's
// last. Skipping a u or C value lets the scan reach a later -S.
func envCluster(words []Word, j int) (cmd string, at int, verdict envVerdict) {
	opt := words[j].Value
	k := strings.IndexAny(opt[1:], "SuC") + 1
	switch {
	case k == 0:
		return "", j, envNext
	case opt[k] != 'S':
		return "", j + boolInt(k == len(opt)-1), envNext
	case k < len(opt)-1:
		return opt[k+1:], j, envFound
	case j+1 >= len(words):
		return "", j, envStop
	}
	return words[j+1].Value, j + 1, envFound
}
