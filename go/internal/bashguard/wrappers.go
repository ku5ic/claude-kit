package bashguard

import (
	"regexp"
	"slices"
	"strings"
)

// wrapperSpec is the grammar of a wrapper, a command that only changes how
// (or as whom) another runs: its options that take a separate value, and
// whether a word after them is its own operand rather than the command.
// whole is for options that are whole words, never short-option clusters
// (arch -arm64e isn't -a -r -m -6 -4 -e).
type wrapperSpec struct {
	values  []string
	operand func(word string) bool
	whole   bool
}

// takes is true when option opt ("-u", "--user") takes a value.
func (s wrapperSpec) takes(opt string) bool { return slices.Contains(s.values, opt) }

// numeric is a number as timeout and chrt read one: 5, 0.5, .5, 5s, inf.
func numeric(word string) bool {
	digits := strings.TrimPrefix(word, ".")
	return digits != "" && digits[0] >= '0' && digits[0] <= '9' || strings.HasPrefix(strings.ToLower(word), "inf")
}

func always(string) bool { return true }

// wrappers are the wrappers lead sees through. busybox and toybox run their
// first word as the applet; flock's first operand is its lock file,
// taskset's its CPU mask or list, script's (macOS) its log file.
var (
	wrappers = map[string]wrapperSpec{
		"command": {}, "builtin": {}, "nohup": {}, "unbuffer": {}, "busybox": {}, "toybox": {},
		"stdbuf":  {values: []string{"-i", "-o", "-e", "--input", "--output", "--error"}},
		"env":     {values: []string{"-u", "-C", "--unset", "--chdir"}},
		"exec":    {values: []string{"-a"}},
		"nice":    {values: []string{"-n", "--adjustment"}},
		"timeout": {values: []string{"-s", "-k", "--signal", "--kill-after"}, operand: numeric},
		"ionice":  {values: []string{"-c", "-n", "--class", "--classdata"}},
		"chrt":    {values: []string{"-p"}, operand: numeric},
		"sudo": {values: []string{"-u", "-g", "-h", "-p", "-C", "-D", "-R", "-T", "-U", "-r", "-t",
			"--user", "--group", "--host", "--prompt", "--close-from", "--chdir", "--chroot", "--command-timeout", "--other-user", "--role", "--type"}},
		"doas":       {values: []string{"-u", "-a", "-C"}},
		"caffeinate": {values: []string{"-t", "-w"}},
		"time":       {values: []string{"-f", "-o", "--format", "--output"}},
		"flock":      {values: []string{"-w", "-E", "--timeout", "--conflict-exit-code"}, operand: always},
		"watch":      {values: []string{"-n", "-q", "--interval", "--equexit"}},
		"setsid":     {},
		"arch":       {values: []string{"-arch", "-d", "-e"}, whole: true},
		"taskset":    {operand: always},
		"script": {values: []string{"-c", "-E", "-I", "-O", "-B", "-T", "-m", "-o", "-t",
			"--command", "--echo", "--log-in", "--log-out", "--log-io", "--log-timing", "--logging-format", "--output-limit"}, operand: always},
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
	// The time keyword takes --, which parses as the command word: time
	// -- rm -rf ~. No command is named --.
	if i+1 < len(words) && words[i].Value == "--" {
		i++
	}
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
	if spec.whole {
		return spec.takes(opt)
	}
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

// runsString is true when a wrapper in words runs a command string, as
// script -c does: like sh -c, the string never surfaces as its own call.
func runsString(words []Word) bool {
	ws, _ := wrappedBy(words)
	for _, w := range ws {
		if w.name != "script" {
			continue
		}
		for _, o := range words[w.at+1 : w.end] {
			if o.Value == "--" {
				break
			}
			if commandOption(o.Value) {
				return true
			}
		}
	}
	return false
}

// commandOption is -c, in a short-option cluster too, or --command, as su
// and script spell the option that runs a string.
func commandOption(v string) bool {
	if name, ok := strings.CutPrefix(v, "--"); ok {
		name, _, _ = strings.Cut(name, "=")
		return name != "" && abbrevOf("--"+name, "--command")
	}
	return len(v) > 1 && v[0] == '-' && strings.ContainsRune(v[1:], 'c')
}

// startsRootShell is true for a call that is only sudo or doas starting a
// shell, with no command after its options (sudo -s, sudo -u root -i), so
// the shell reads its commands from stdin.
func startsRootShell(words []Word) bool {
	i := lead(words)
	return i == len(words)-1 && strings.HasPrefix(words[i].Value, "-") && rootShell(words)
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
