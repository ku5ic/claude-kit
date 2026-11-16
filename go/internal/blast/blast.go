// Package blast is blast-radius: the files that use a given file
// (skills/kit-rules/reference/change.md section 3). Who uses it is
// codegraph.Importers; this package splits them into source and test,
// keeps the ones using a symbol as a whole word, and prints them.
package blast

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/codegraph"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/guard"
)

type scan struct {
	root, rel, symbol string
	testGlobs         []string
	seen              map[string]bool
	results           []string
	tests             int
}

// Run is `kit blast-radius <file> [symbol]`.
func Run(cfg *config.Config, args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintln(stderr, "usage: kit blast-radius <file> [symbol]")
		return 2
	}
	target, symbol := args[0], ""
	if len(args) == 2 {
		symbol = args[1]
	}
	if !fsx.IsFile(target) {
		fmt.Fprintf(stderr, "blast-radius: no such file: %s\n", target)
		return 2
	}
	// Absolute before physical: a relative dir stays relative to cwd, not
	// the repo root the rel path below is cut from.
	dir, _ := filepath.Abs(filepath.Dir(target))
	dir, _ = filepath.EvalSymlinks(dir)
	abs := filepath.Join(dir, filepath.Base(target))
	root := git.Toplevel(dir)
	if root == "" {
		fmt.Fprintln(stderr, "blast-radius: not inside a git repository")
		return 2
	}
	s := &scan{root: root, rel: fsx.Rel(root, abs), symbol: symbol, seen: map[string]bool{}}
	if !codegraph.Scans(s.rel) {
		fmt.Fprintf(stderr, "blast-radius: no import scanner for %s\n", filepath.Base(s.rel))
		return 2
	}
	uses, err := codegraph.Importers(root, s.rel, cfg.SkipDirs, stderr)
	switch {
	case errors.Is(err, codegraph.ErrNoASTGrep):
		fmt.Fprintf(stderr, "blast-radius: %s needs ast-grep: %v\n", filepath.Base(s.rel), err)
		return 2
	case err != nil:
		fmt.Fprintf(stderr, "blast-radius: %v\n", err)
		return 2
	}
	for _, rule := range cfg.SkillFileMap {
		if slices.Contains(rule.Skills, "test-patterns") {
			s.testGlobs = append(s.testGlobs, rule.Globs...)
		}
	}
	for _, imp := range uses.Importers {
		s.add(imp.File, imp.Line, imp.Label)
	}
	s.print(uses.Unresolvable, stdout)
	return 0
}

// print writes the consumer list, the first 50 sorted.
func (s *scan) print(unresolvable bool, stdout io.Writer) {
	header := "blast-radius: " + s.rel
	if s.symbol != "" {
		header += " (symbol " + s.symbol + ")"
	}
	fmt.Fprintln(stdout, header)
	fmt.Fprintf(stdout, "consumers: %d (source %d, test %d)\n", len(s.results), len(s.results)-s.tests, s.tests)
	slices.Sort(s.results)
	for _, r := range s.results[:min(len(s.results), 50)] {
		fmt.Fprintln(stdout, r)
	}
	if len(s.results) > 50 {
		fmt.Fprintf(stdout, "... %d more\n", len(s.results)-50)
	}
	if unresolvable {
		fmt.Fprintln(stdout, "unresolvable imports present")
	}
}

var wordChars = regexp.MustCompile(`[A-Za-z0-9_]`)

func (s *scan) add(file string, line int, label string) {
	if file == s.rel || s.seen[file] {
		return
	}
	s.seen[file] = true
	if s.symbol != "" && !containsWord(filepath.Join(s.root, file), s.symbol) {
		return
	}
	kind := "source"
	if guard.GlobAny(s.testGlobs, filepath.Base(file)) {
		kind = "test"
		s.tests++
	}
	result := fmt.Sprintf("%s:%d %s", file, line, kind)
	if label != "" {
		result += " " + label
	}
	s.results = append(s.results, result)
}

// containsWord is grep -qw: word as a whole word, word characters being
// letters, digits, and underscore.
func containsWord(file, word string) bool {
	data, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	text := string(data)
	for i := 0; ; {
		j := strings.Index(text[i:], word)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(word)
		before := start == 0 || !wordChars.MatchString(text[start-1:start])
		after := end == len(text) || !wordChars.MatchString(text[end:end+1])
		if before && after {
			return true
		}
		i = start + 1
	}
}
