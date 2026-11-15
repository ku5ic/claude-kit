package codegraph

import (
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/git"
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
// the other, and both paths. A file root no longer tracks, or one under
// skipDirs or testdata, as imports and deps skip it, is in no pair.
func Coupling(gitLog, resolved, root string, skipDirs []string, out io.Writer) error {
	commits, err := readCommits(gitLog)
	if err != nil {
		return err
	}
	files, err := git.Paths(root, "ls-files", "-z")
	if err != nil {
		return fmt.Errorf("git ls-files: %w", err)
	}
	tracked := map[string]bool{}
	for _, f := range files {
		tracked[f] = !skipped(f, skipDirs)
	}
	changes, pairs := map[string]int{}, map[pair]int{}
	for _, c := range commits {
		if c.size > maxFilesPerCommit {
			continue
		}
		files := slices.DeleteFunc(c.files, func(f string) bool { return !tracked[f] })
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

// commit is one commit's changed files, minus noise and tests, and how
// many files it changed before those went.
type commit struct {
	files []string
	size  int
}

// readCommits is each commit in gitLog.
func readCommits(gitLog string) ([]commit, error) {
	rows, err := readLines(gitLog)
	var commits []commit
	for _, line := range rows {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "--"):
			commits = append(commits, commit{})
		case line != "" && len(commits) > 0:
			c := &commits[len(commits)-1]
			c.size++
			if !couplingNoise.MatchString(line) && !testOrStory.MatchString(line) {
				c.files = append(c.files, line)
			}
		}
	}
	return commits, err
}
