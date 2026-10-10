package main

import (
	"errors"
	"go/build"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// layers is each package's layer. A package imports only from lower
// layers, so a change to one never ripples upward. testutil has no layer:
// only test files may import it.
var layers = map[string]int{
	"internal/fsx":        0,
	"internal/proc":       0,
	"internal/md":         0,
	"internal/config":     1,
	"internal/cache":      1,
	"internal/git":        1,
	"internal/transcript": 1,
	"internal/kitcmd":     1,
	"internal/kitlog":     1,
	"internal/project":    2,
	"internal/guard":      2,
	"internal/sources":    2,
	"internal/hook":       3,
	"internal/resolve":    3,
	"internal/classify":   4,
	"internal/gapfill":    4,
	"internal/tools":      4,
	"internal/checks":     5,
	"internal/enforce":    5,
	"internal/bashguard":  5,
	"internal/hooks":      5,
	"internal/stackctx":   5,
	"internal/explain":    6,
	"internal/rotate":     6,
	"internal/blast":      6,
	"internal/status":     6,
	"internal/gitbase":    6,
	"internal/detect":     6,
	"cmd/kit":             7,
}

// allowed are the same-layer and upward imports that exist today. The
// step that removes one deletes its entry; a stale entry fails the test.
var allowed = map[string]bool{
	"internal/project -> internal/sources": true,
	"internal/tools -> internal/classify":  true,
	"internal/hooks -> internal/checks":    true,
	"internal/hooks -> internal/stackctx":  true,
	"internal/stackctx -> internal/detect": true,
}

func TestImportsGoDown(t *testing.T) {
	t.Parallel()
	const module = "github.com/ku5ic/claude-kit/go/"
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	err = filepath.WalkDir(root, func(dir string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		if d.Name() == "testdata" {
			return filepath.SkipDir // fixtures, never built
		}
		pkg, err := build.ImportDir(dir, 0)
		if _, none := errors.AsType[*build.NoGoError](err); none {
			return nil
		}
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, dir)
		if rel == "internal/testutil" || strings.HasPrefix(rel, "e2e") {
			return nil
		}
		layer, ok := layers[rel]
		if !ok {
			t.Errorf("%s has no layer", rel)
			return nil
		}
		for _, imp := range pkg.Imports {
			dep, ok := strings.CutPrefix(imp, module)
			if !ok {
				continue
			}
			edge := rel + " -> " + dep
			depLayer, known := layers[dep]
			switch {
			case !known:
				t.Errorf("%s: %s has no layer", edge, dep)
			case depLayer >= layer && allowed[edge]:
				used[edge] = true
			case depLayer >= layer:
				t.Errorf("%s: layer %d imports layer %d", edge, layer, depLayer)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for edge := range allowed {
		if !used[edge] {
			t.Errorf("%s is allowed but no longer imported; delete the entry", edge)
		}
	}
}
