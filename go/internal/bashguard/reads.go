package bashguard

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"

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

// rgReadsOnly is whether every option in args, an rg call's words after
// its name, is one rgFlags lists. A value an option takes, in its word or
// the next, isn't read as an option.
func rgReadsOnly(args []string) bool {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return true
		case strings.HasPrefix(a, "--no-"):
		case strings.HasPrefix(a, "--"):
			name, _, inline := strings.Cut(a[2:], "=")
			value, ok := rgFlags[name]
			if !ok || len(name) == 1 {
				return false
			}
			if value && !inline {
				i++
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			for j, r := range a[1:] {
				value, ok := rgFlags[string(r)]
				if !ok {
					return false
				}
				// -A3 or -e x: the value is the rest of the word, or the next.
				if value {
					if j == len(a)-2 {
						i++
					}
					break
				}
			}
		}
	}
	return true
}

// ReadTargets is what cmd, run from cwd, reads, as guard-bash's credential
// check counts reads: each existing file a command's operands or < redirects
// name, and each glob a grep or rg is limited to, its {a,b} alternatives
// expanded and joined to the directory it runs in.
func ReadTargets(cmd, home, cwd string) []string {
	segs, _ := parseAll(cmd)
	var out []string
	for w := range walk(segs, home, resolveDir(home, "/", cwd)) {
		values := make([]string, 0, len(w.call.Words)-w.i-1)
		for _, word := range w.call.Words[w.i+1:] {
			values = append(values, word.Value)
		}
		for _, op := range append(readOperands(w.name, values), w.call.Inputs...) {
			if path := resolveDir(home, w.cwd, op); fsx.IsFile(path) {
				out = append(out, path)
			}
		}
		ghFlags(values, globFlags, func(_, glob string) {
			alts, _ := guard.Braces(glob)
			for _, alt := range alts {
				out = append(out, filepath.Join(w.cwd, alt))
			}
		})
	}
	return out
}

// pureReaders print or test what they're given and write no file,
// whatever their flags. Commands that write only with a flag (sed -i,
// sort -o, find -delete) aren't here.
var pureReaders = map[string]bool{
	"cat": true, "head": true, "tail": true, "wc": true, "ls": true, "stat": true,
	"realpath": true, "readlink": true, "test": true, "[": true, "echo": true, "printf": true,
	"rg": true, "grep": true, "diff": true, "cmp": true, "cut": true, "nl": true, "basename": true,
	"dirname": true, "du": true, "tokei": true, "shasum": true, "jq": true, "gron": true, "pwd": true,
	"true": true,
}

// readerWriteFlags are the flags that make a reader write a file or run a
// command: rg's preprocessor and hostname program, git's output file,
// external diff, and pager.
var readerWriteFlags = []string{"--pre", "--hostname-bin", "--output", "--ext-diff", "--textconv", "-O", "--open-files-in-pager"}

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
	for _, seg := range segs {
		for _, c := range seg.Calls {
			if len(c.Words) == 0 || len(c.Assigns) > 0 {
				return false
			}
			for _, r := range c.Redirs {
				if r.Target.Value != "/dev/null" && !strings.HasPrefix(r.Target.Value, "&") {
					return false
				}
			}
			// A path is a script of the project's: ./test or scripts/diff
			// can write whatever it likes.
			name, args := c.Words[0].Value, c.Words[1:]
			if strings.Contains(name, "/") {
				return false
			}
			for name == "git" && len(args) > 1 && args[0].Value == "-C" {
				args = args[2:]
			}
			if slices.ContainsFunc(args, func(w Word) bool {
				return slices.ContainsFunc(readerWriteFlags, func(f string) bool { return strings.HasPrefix(w.Value, f) })
			}) {
				return false
			}
			switch {
			case pureReaders[name]:
			case name == "git" && len(args) > 0 && isGitRead(args[0].Value):
			case name == "kit" && len(args) > 0 && kitcmd.ReadOnly(args[0].Value):
			default:
				return false
			}
		}
	}
	return true
}

// excludeSensitive gives a lone rg with only literal words and read-only
// flags (rgFlags), reading only inside the work tree it runs in, the
// sensitive exclusion as its first
// flag, so a loose .env or *.pem under it isn't searched. It returns the
// rewritten command, or "" when it left cmd alone. The allow keeps auto mode from denying the
// rewrite, and approves only this read; the user's deny and ask rules still
// apply. A chained or piped rg keeps the shortcut at guard.SearchesSensitive.
func (st *state) excludeSensitive(segs []Segment, cmd string) string {
	exclude := guard.SensitiveExclusion(st.cfg)
	rest, ok := strings.CutPrefix(strings.TrimSpace(cmd), "rg ")
	// onlyReads turns away a flag that runs a program (--pre, --hostname-bin),
	// which the allow would approve.
	if exclude == "" || !ok || len(segs) != 1 || len(segs[0].Calls) != 1 || segs[0].subshell || !onlyReads(segs) {
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
	if !rgReadsOnly(values) {
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
	st.h.Decide("allow", "kit excluded credential files from an rg inside the project")
	return rewritten
}
