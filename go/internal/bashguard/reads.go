package bashguard

import (
	"path/filepath"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/guard"
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
