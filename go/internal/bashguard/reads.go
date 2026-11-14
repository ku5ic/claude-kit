package bashguard

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/kitcmd"
)

// globFlags are grep's and rg's options whose value limits the search to a
// file glob.
var globFlags = map[string]string{"-g": "glob", "--glob": "glob", "--iglob": "glob", "--include": "glob"}

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
