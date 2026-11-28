package bashguard

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitcmd"
)

// globFlags are grep's and rg's options whose value limits the search to a
// file glob.
var globFlags = map[string]string{"-g": "glob", "--glob": "glob", "--iglob": "glob", "--include": "glob"}

// rgFlags are the rg options (rg 15, rg --help) known to change only what
// is matched or how it prints, each true when it takes a value. Missing on
// purpose: -L and --follow leave the project, -g, --glob, and --iglob
// replace kit's glob, -z runs decompressors, --pre and --hostname-bin run
// a program, -f and --ignore-file read a file of their own. A --no- flag
// only turns one off.
var rgFlags = map[string]bool{
	"s": false, "F": false, "i": false, "v": false, "x": false, "U": false, "P": false, "S": false,
	"a": false, "w": false, ".": false, "u": false, "b": false, "n": false, "N": false, "0": false,
	"o": false, "p": false, "q": false, "H": false, "I": false, "c": false, "l": false, "h": false, "V": false,
	"e": true, "E": true, "m": true, "j": true, "d": true, "t": true, "T": true, "A": true, "B": true,
	"C": true, "M": true, "r": true,
	"case-sensitive": false, "crlf": false, "fixed-strings": false, "ignore-case": false, "invert-match": false,
	"line-regexp": false, "mmap": false, "multiline": false, "multiline-dotall": false, "null-data": false,
	"pcre2": false, "smart-case": false, "stop-on-nonmatch": false, "text": false, "word-regexp": false,
	"auto-hybrid-regex": false, "binary": false, "glob-case-insensitive": false, "hidden": false,
	"ignore-file-case-insensitive": false, "one-file-system": false, "unrestricted": false,
	"block-buffered": false, "byte-offset": false, "column": false, "heading": false, "help": false,
	"include-zero": false, "line-buffered": false, "line-number": false, "max-columns-preview": false,
	"null": false, "only-matching": false, "passthru": false, "pretty": false, "quiet": false, "trim": false,
	"vimgrep": false, "with-filename": false, "sort-files": false, "count": false, "count-matches": false,
	"files-with-matches": false, "files-without-match": false, "json": false, "debug": false, "stats": false,
	"trace": false, "files": false, "pcre2-version": false, "type-list": false, "version": false,
	"regexp": true, "dfa-size-limit": true, "encoding": true, "engine": true, "max-count": true,
	"regex-size-limit": true, "threads": true, "max-depth": true, "max-filesize": true, "type": true,
	"type-not": true, "type-add": true, "type-clear": true, "after-context": true, "before-context": true,
	"color": true, "colors": true, "context": true, "context-separator": true, "field-context-separator": true,
	"field-match-separator": true, "hyperlink-format": true, "max-columns": true, "path-separator": true,
	"replace": true, "sort": true, "sortr": true, "generate": true,
}

// stdinFilters are the commands that, given no operand, read only their
// stdin, each with the options (coreutils and BSD) known not to write a
// file, read one, or run a program, true when it takes a value. Missing on
// purpose: sort -o, -T, --compress-program, --files0-from, and
// --random-source, wc --files0-from, and tail -f.
var stdinFilters = map[string]map[string]bool{
	"head": withDigits(map[string]bool{"n": true, "c": true, "q": false, "v": false, "z": false,
		"lines": true, "bytes": true, "quiet": false, "silent": false, "verbose": false, "zero-terminated": false}),
	"tail": withDigits(map[string]bool{"n": true, "c": true, "q": false, "v": false, "z": false, "r": false,
		"lines": true, "bytes": true, "quiet": false, "silent": false, "verbose": false, "zero-terminated": false}),
	"wc": {"l": false, "w": false, "c": false, "m": false, "L": false,
		"lines": false, "words": false, "bytes": false, "chars": false, "max-line-length": false},
	"sort": {"b": false, "d": false, "f": false, "g": false, "h": false, "i": false, "M": false, "n": false,
		"R": false, "r": false, "V": false, "c": false, "C": false, "m": false, "s": false, "u": false, "z": false,
		"k": true, "t": true, "S": true,
		"ignore-leading-blanks": false, "dictionary-order": false, "ignore-case": false, "general-numeric-sort": false,
		"human-numeric-sort": false, "ignore-nonprinting": false, "month-sort": false, "numeric-sort": false,
		"random-sort": false, "reverse": false, "version-sort": false, "check": false, "merge": false, "stable": false,
		"unique": false, "zero-terminated": false, "key": true, "field-separator": true, "buffer-size": true,
		"parallel": true, "sort": true},
	"uniq": {"c": false, "d": false, "u": false, "i": false, "D": false, "z": false, "f": true, "s": true, "w": true,
		"count": false, "repeated": false, "unique": false, "ignore-case": false, "zero-terminated": false,
		"all-repeated": false, "group": false, "skip-fields": true, "skip-chars": true, "check-chars": true},
	"cut": {"s": false, "n": false, "z": false, "w": false, "b": true, "c": true, "f": true, "d": true,
		"only-delimited": false, "complement": false, "zero-terminated": false, "bytes": true, "characters": true,
		"fields": true, "delimiter": true, "output-delimiter": true},
}

// withDigits adds head's and tail's -5 to flags.
func withDigits(flags map[string]bool) map[string]bool {
	for d := '0'; d <= '9'; d++ {
		flags[string(d)] = false
	}
	return flags
}

// knownOptions splits args, a call's words after its name, into its
// operands, false when an option isn't in flags. A value an option takes,
// in its word or the next, isn't read as an option. With negatable, any
// --no- flag passes: it only turns one off.
func knownOptions(flags map[string]bool, negatable bool, args []string) ([]string, bool) {
	var operands []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return append(operands, args[i+1:]...), true
		case negatable && strings.HasPrefix(a, "--no-"):
		case strings.HasPrefix(a, "--"):
			name, _, inline := strings.Cut(a[2:], "=")
			value, ok := flags[name]
			if !ok || len(name) == 1 {
				return nil, false
			}
			if value && !inline {
				i++
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			known, next := clusterOptions(flags, a)
			if !known {
				return nil, false
			}
			i += boolInt(next)
		default:
			operands = append(operands, a)
		}
	}
	return operands, true
}

// clusterOptions reads a short cluster against flags: whether each letter
// up to the first that takes a value is known, and whether that value is
// the next word. -A3 or -e x: the value is the rest of the word, or the
// next.
func clusterOptions(flags map[string]bool, a string) (known, next bool) {
	for j, r := range a[1:] {
		value, ok := flags[string(r)]
		if !ok {
			return false, false
		}
		if value {
			return true, j == len(a)-2
		}
	}
	return true, false
}

// filtersStdin is whether c reads only its stdin and writes only to
// stdout: a stdinFilters command with literal, known options and no
// operand, input, or redirect but a discarding one.
func filtersStdin(c Call) bool {
	if len(c.Words) == 0 || len(c.Assigns) > 0 || len(c.Inputs) > 0 || len(c.Heredocs) > 0 || !discards(c.Redirs) {
		return false
	}
	flags, ok := stdinFilters[c.Words[0].Value]
	if !ok {
		return false
	}
	args := make([]string, 0, len(c.Words)-1)
	for _, w := range c.Words[1:] {
		if !w.literal {
			return false
		}
		args = append(args, w.Value)
	}
	operands, ok := knownOptions(flags, false, args)
	return ok && len(operands) == 0
}

// discards is whether every redirect goes to /dev/null or another
// descriptor.
func discards(redirs []Redir) bool {
	return !slices.ContainsFunc(redirs, func(r Redir) bool {
		return r.Target.Value != "/dev/null" && !strings.HasPrefix(r.Target.Value, "&")
	})
}

// ReadTargets is what cmd, run from cwd, reads, as guard-bash's credential
// check counts reads: each existing file a command's operands or < redirects
// name, and each glob a grep or rg is limited to, its {a,b} alternatives
// expanded and joined to the directory it runs in.
func ReadTargets(cmd, home, cwd, shell string) []string {
	segs, _ := parseAll(cmd, shell)
	var out []string
	for w := range walk(segs, home, resolveDir(home, "/", cwd)) {
		args := values(w.call.Words[w.i+1:])
		for _, op := range append(readOperands(w.name, args), w.call.Inputs...) {
			if path := resolveDir(home, w.cwd, op); fsx.IsFile(path) {
				out = append(out, path)
			}
		}
		walkFlags(args, globFlags, func(_, glob string) {
			if strings.HasPrefix(glob, "!") {
				return
			}
			alts, _ := guard.Braces(glob)
			for _, alt := range alts {
				out = append(out, filepath.Join(w.cwd, alt))
			}
		})
	}
	return out
}

// gitReads are the git subcommands that only read, each true when it
// prints the files it names: git ls-files .env names the file, show
// HEAD:.env prints it.
var gitReads = map[string]bool{
	"log": true, "diff": true, "show": true, "grep": true, "blame": true, "annotate": true, "cat-file": true,
	"status": false, "rev-parse": false, "ls-files": false,
}

// isGitRead is true when git sub only reads.
func isGitRead(sub string) bool {
	_, ok := gitReads[sub]
	return ok
}

// onlyReads is true when every command in segs only reads: a pure reader,
// a reading git subcommand, or a read-only kit subcommand, with output
// redirected nowhere but /dev/null or another descriptor.
func onlyReads(segs []Segment) bool {
	return !slices.ContainsFunc(segs, func(seg Segment) bool {
		return slices.ContainsFunc(seg.Calls, func(c Call) bool { return !callOnlyReads(c) })
	})
}

// callOnlyReads is onlyReads for one call. A path is a script of the
// project's: ./test or scripts/diff can write whatever it likes.
func callOnlyReads(c Call) bool {
	if len(c.Words) == 0 || len(c.Assigns) > 0 || !discards(c.Redirs) {
		return false
	}
	name, args := c.Words[0].Value, c.Words[1:]
	if strings.Contains(name, "/") {
		return false
	}
	for name == "git" && len(args) > 1 && args[0].Value == "-C" {
		args = args[2:]
	}
	return !slices.ContainsFunc(args, func(w Word) bool { return classify.WritingFlag(w.Value) }) && readsByName(name, args)
}

// readsByName is whether name with args only reads: a pure reader, a
// reading git subcommand, or a read-only kit subcommand.
func readsByName(name string, args []Word) bool {
	switch {
	case classify.PureReaders[name]:
		return true
	case len(args) == 0:
		return false
	case name == "git":
		return isGitRead(args[0].Value)
	case name == "kit":
		return kitcmd.ReadOnly(args[0].Value)
	}
	return false
}

// excludeSensitive gives an rg with only literal words and read-only flags
// (rgFlags), reading only inside the work tree it runs in, alone or piped
// only into stdinFilters, the sensitive exclusion as its first flag, so a
// loose .env or *.pem under it isn't searched. It returns the rewritten
// command, or "" when it left cmd alone. The allow keeps auto mode from
// denying the rewrite, and approves only this read; the user's deny and ask
// rules still apply. A chained rg keeps the shortcut at
// guard.SearchesSensitive.
func (st *state) excludeSensitive(segs []Segment, cmd string) string {
	exclude := guard.SensitiveExclusion(st.cfg)
	rest, ok := strings.CutPrefix(strings.TrimSpace(cmd), "rg ")
	if exclude == "" || !ok || len(segs) != 1 || segs[0].subshell {
		return ""
	}
	// onlyReads turns away a flag that runs a program (--pre, --hostname-bin),
	// which the allow would approve.
	search := segs[0]
	search.Calls = search.Calls[:1]
	if !onlyReads([]Segment{search}) || slices.ContainsFunc(segs[0].Calls[1:], func(c Call) bool { return !filtersStdin(c) }) {
		return ""
	}
	top := st.worktreeTop()
	if top == "" {
		return ""
	}
	var values []string
	for _, w := range segs[0].Calls[0].Words[1:] {
		// "$OLDPWD" or {~/.aws,.} reads as a path under cwd, so only a
		// literal word gets the allow.
		if !w.literal {
			return ""
		}
		values = append(values, w.Value)
	}
	// Only a flag known to read gets the allow; any other leaves the
	// command to the permission rules.
	if _, ok := knownOptions(rgFlags, true, values); !ok {
		return ""
	}
	// rg searches a file it's named whatever the globs say.
	if slices.ContainsFunc(readOperands("rg", values), func(p string) bool {
		return guard.IsSensitive(st.cfg, st.home, st.abs(unquote(p)))
	}) {
		return ""
	}
	// The allow approves only a read inside the project. The pattern and
	// any option value count as operands too; inside, they're harmless.
	for _, operand := range nonOptions(values) {
		if !fsx.Within(top, resolveDir(st.home, st.cwd, operand)) {
			return ""
		}
	}
	rewritten := "rg --glob '" + exclude + "' " + rest
	input := maps.Clone(st.h.Payload.Object(hook.KeyToolInput))
	input["command"] = rewritten
	st.h.UpdateInput(input)
	st.h.Allow("kit excluded credential files from an rg inside the project")
	return rewritten
}
