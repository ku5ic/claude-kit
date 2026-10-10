package enforce

import (
	"fmt"
	"io"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
)

// Print writes p as --plan shows it: each gate as RUN with its command,
// where it runs, its binaries, and its source's verdict, or as SKIP with
// why; a project that states nothing gets the discovery instruction.
func (p Plan) Print(w io.Writer) {
	if p.Discovery != "" {
		fmt.Fprintf(w, "SKIP everything (%s)\n", p.Discovery)
		return
	}
	for _, g := range p.Gates {
		if g.Skip != "" {
			fmt.Fprintf(w, "SKIP %s (%s)\n", g.Label, g.Skip)
			continue
		}
		fmt.Fprintf(w, "RUN %s\n", g.Label)
		p.Describe(w, g)
		fmt.Fprintf(w, "  verdict: %s\n", g.Verdict)
		if g.Scoped() {
			fmt.Fprint(w, "  scope: only findings on lines changed since the git base fail it\n")
		}
	}
}

// Describe writes, indented under g's line, its command, where it runs
// unless at the root, its env, and its binaries.
func (p Plan) Describe(w io.Writer, g Gate) {
	fmt.Fprintf(w, "  cmd: %s\n", strings.ReplaceAll(strings.TrimSpace(g.Body), "\n", "\n       "))
	if g.Dir != p.Root {
		fmt.Fprintf(w, "  dir: %s\n", fsx.Rel(p.Root, g.Dir))
	}
	if len(g.Env) > 0 {
		fmt.Fprintf(w, "  env: %s\n", strings.Join(g.Env, " "))
	}
	for _, bin := range g.Bins {
		fmt.Fprintf(w, "  bin: %s (%s)\n", bin.Path, bin.Source)
		if bin.Note != "" && bin.Source != "toolchain" {
			fmt.Fprintf(w, "  note: %s\n", bin.Note)
		}
	}
}
