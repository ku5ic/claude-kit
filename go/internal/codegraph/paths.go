// Package codegraph is code-graph: a repo's imports per file (imports),
// the module edges, externals, and upward imports they make (deps), and
// the files that change together (coupling), each as TSV.
package codegraph

import (
	"cmp"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
)

// testOrStory is a test, fixture, mock, or story file: deps and coupling
// both keep them out of what they count as production.
var testOrStory = regexp.MustCompile(`(^|/)(tests?|spec|__tests__|mocks?|testdata|fixtures|stories)/` +
	`|[._-](test|spec|stories)\.[^/]+$|(^|/)test_[^/]+$|Tests?\.[A-Za-z]+$`)

// dirFiles is the basename that stands for its directory, by ecosystem.
var dirFiles = map[string]string{"npm": "index", "python": "__init__", "rust": "mod"}

// moduleStem is what an import of p names: no extension, and no basename
// its language reads as the directory (src/mod/index.ts is src/mod).
func moduleStem(p string) string {
	ext := path.Ext(p)
	stem := strings.TrimSuffix(p, ext)
	// A root index.ts stays index, since the root is no module.
	if dir, base := path.Split(stem); dir != "" && base == dirFiles[ecosystems[ext]] {
		return strings.TrimSuffix(dir, "/")
	}
	return stem
}

// moduleID is one name for p, whether p is a tracked file or the stem an
// import resolved to: a file by its moduleStem, a stem whole, since its
// dot (b.client) is no extension.
func moduleID(p string, tracked map[string]bool) string {
	if tracked[p] {
		return moduleStem(p)
	}
	return p
}

// dirOf is p's directory, "" at the repo root.
func dirOf(p string) string {
	if d := path.Dir(p); d != "." {
		return d
	}
	return ""
}

// byCount is m's keys, highest count first, ties by order.
func byCount[K comparable](m map[K]int, order func(a, b K) int) []K {
	return slices.SortedFunc(maps.Keys(m), func(a, b K) int {
		return cmp.Or(cmp.Compare(m[b], m[a]), order(a, b))
	})
}

// pair is two paths: an import edge between modules, or two files that
// change together.
type pair struct{ a, b string }

func (p pair) compare(q pair) int {
	return cmp.Or(strings.Compare(p.a, q.a), strings.Compare(p.b, q.b))
}
