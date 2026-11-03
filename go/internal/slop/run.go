package slop

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

const usage = "usage: kit slop [--surface file|commit|pr|report|chat] <file...|->"

// Run is `kit slop`: every hit in each file, or stdin, one line each. It
// exits 1 when any hit blocks, 2 on a usage error.
func Run(cfg *config.Config, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	surface, paths, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "kit slop: %v\n%s\n", err, usage)
		return 2
	}
	d := New(cfg)
	blocked := false
	for _, path := range paths {
		var data []byte
		if path == "-" {
			data, err = io.ReadAll(stdin)
		} else if abs, _ := filepath.Abs(path); d.Skips(abs) {
			fmt.Fprintf(stderr, "%s: skipped (slop.skip)\n", path)
			continue
		} else {
			data, err = os.ReadFile(path)
		}
		if err != nil {
			fmt.Fprintln(stderr, "kit slop:", err)
			return 2
		}
		segs, allows, ok := Segments(path, "", string(data))
		if !ok {
			fmt.Fprintf(stderr, "%s: not scanned (no reader for this file type)\n", path)
			continue
		}
		for _, a := range allows {
			fmt.Fprintf(stdout, "%s:%d: allow %q\n", path, a.Line, a.Reason)
		}
		res := d.Scan(segs, surface)
		for _, h := range res.Hits {
			line := fmt.Sprintf("%s:%d: T%d %s %q", path, h.Line, h.Tier, h.Rule, h.Match)
			if h.Fix != "" {
				line += " -> " + h.Fix
			}
			fmt.Fprintln(stdout, line)
			blocked = blocked || d.Blocks(h)
		}
		if res.Dense {
			fmt.Fprintf(stdout, "%s: tier 3 density %.1f per %d words\n", path, res.Score, cfg.Slop.Density.PerWords)
		}
	}
	if blocked {
		return 1
	}
	return 0
}

func parseArgs(args []string) (surface string, paths []string, err error) {
	surface = "file"
	explicit := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--surface":
			if i+1 == len(args) {
				return "", nil, fmt.Errorf("--surface needs a value")
			}
			i++
			surface, explicit = args[i], true
		case strings.HasPrefix(a, "--surface="):
			surface, explicit = strings.TrimPrefix(a, "--surface="), true
		case a != "-" && strings.HasPrefix(a, "-"):
			return "", nil, fmt.Errorf("unknown flag %s", a)
		default:
			paths = append(paths, a)
		}
	}
	switch {
	case len(paths) == 0:
		return "", nil, fmt.Errorf("no file")
	case !slices.Contains(config.SlopSurfaces, surface):
		return "", nil, fmt.Errorf("%q is no surface", surface)
	case slices.Contains(paths, "-") && !explicit:
		return "", nil, fmt.Errorf("stdin needs --surface")
	}
	return surface, paths, nil
}
