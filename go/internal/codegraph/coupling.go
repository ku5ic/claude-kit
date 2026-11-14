package codegraph

import (
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"strings"
)

const (
	// maxFilesPerCommit skips bulk commits: a rename or a format pass
	// couples everything it touches.
	maxFilesPerCommit = 30
	// minShared is how many commits a pair shares before it counts.
	minShared = 5
)

// couplingNoise changes alongside everything: lock files, manifests, CI,
// docs, and generated files. Data files stay in: flags.json and
// routes.yaml are the registries hidden coupling exposes.
var couplingNoise = regexp.MustCompile(`\.snap$|\.lock$|-lock\.|go\.(sum|mod)$|generated|\.(md|svg|css|scss|txt)$|(^|/)\.github/` +
	`|(^|/)(package|composer|tsconfig[^/]*)\.json$|(^|/)(Cargo|pyproject)\.toml$|(^|/)pubspec\.yaml$`)

// Coupling reads gitLog, from git log --format='--%H' --name-only, and
// resolved, the imports-resolved.tsv Deps writes, and writes to out each
// pair of files in different directories that changed together at least
// minShared times, under a header row: shared commits, coupling degree
// (shared over the less-changed file's commits), whether either imports
// the other, and both paths.
func Coupling(gitLog, resolved string, out io.Writer) error {
	commits, err := readCommits(gitLog)
	if err != nil {
		return err
	}
	changes, pairs := map[string]int{}, map[pair]int{}
	for _, files := range commits {
		if len(files) > maxFilesPerCommit {
			continue
		}
		for _, f := range files {
			changes[f]++
		}
		slices.Sort(files)
		files = slices.Compact(files)
		for i, a := range files {
			for _, b := range files[i+1:] {
				if dirOf(a) != dirOf(b) {
					pairs[pair{a, b}]++
				}
			}
		}
	}
	rows, err := readTSV(resolved, 2)
	if err != nil {
		return err
	}
	imports := map[pair]bool{}
	for _, r := range rows {
		// A Go file's target is its package directory, kept as importTarget
		// reads it.
		target := r[1]
		if path.Ext(r[0]) != ".go" {
			target = moduleStem(target)
		}
		imports[pair{moduleStem(r[0]), target}] = true
	}
	fmt.Fprintln(out, "shared\tdegree\tlink\tfile_a\tfile_b")
	for _, p := range byCount(pairs, pair.compare) {
		shared := pairs[p]
		if shared < minShared {
			break
		}
		link := "no-import"
		if imports[pair{moduleStem(p.a), importTarget(p.b)}] || imports[pair{moduleStem(p.b), importTarget(p.a)}] {
			link = "imports"
		}
		degree := float64(shared) / float64(min(changes[p.a], changes[p.b]))
		fmt.Fprintf(out, "%d\t%.2f\t%s\t%s\t%s\n", shared, degree, link, p.a, p.b)
	}
	return nil
}

// importTarget is how imports-resolved.tsv names file as an import's
// target: a Go file by its package's directory, any other by its module stem.
func importTarget(file string) string {
	if path.Ext(file) == ".go" {
		return dirOf(file)
	}
	return moduleStem(file)
}

// readCommits is each commit's changed files, minus noise and tests.
func readCommits(gitLog string) ([][]string, error) {
	rows, err := readLines(gitLog)
	var commits [][]string
	for _, line := range rows {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "--"):
			commits = append(commits, nil)
		case line != "" && len(commits) > 0 && !couplingNoise.MatchString(line) && !testOrStory.MatchString(line):
			commits[len(commits)-1] = append(commits[len(commits)-1], line)
		}
	}
	return commits, err
}
