package codegraph

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const usage = `usage: kit code-graph imports <root> > imports.tsv
       kit code-graph deps <root> <imports.tsv> <out dir> [split dir]
       kit code-graph coupling <git log> <imports-resolved.tsv> <root> > coupling.tsv
       kit code-graph model-dir <repo or subdir>
       kit code-graph fingerprint <deps out dir> > fingerprint.txt
       kit code-graph c4 <deps out dir> <system name> > c4-containers.mmd
       kit code-graph check-c4 <model dir> <deps out dir> <root>`

// Run is kit code-graph: its first argument names the report. imports and
// deps leave out a path under any of skipDirs.
func Run(args, skipDirs []string, stdout, stderr io.Writer) int {
	var r report
	if len(args) > 0 {
		r = reports[args[0]]
	}
	if r.run == nil || !slices.Contains(r.arity, len(args)) {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	status, err := r.run(args, skipDirs, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "code-graph %s: %v\n", args[0], err)
		return 1
	}
	return status
}

// report is one kit code-graph report: the arg counts it takes, the report
// name included, and what runs it, giving its exit status.
type report struct {
	arity []int
	run   func(args, skipDirs []string, stdout, stderr io.Writer) (int, error)
}

// reports are kit code-graph's reports by name.
var reports = map[string]report{
	"imports": {[]int{2}, func(args, skipDirs []string, stdout, stderr io.Writer) (int, error) {
		return 0, Imports(args[1], skipDirs, stdout, stderr)
	}},
	"deps": {[]int{4, 5}, func(args, skipDirs []string, stdout, _ io.Writer) (int, error) {
		split := ""
		if len(args) == 5 {
			split = args[4]
		}
		return 0, Deps(args[1], args[2], args[3], split, skipDirs, stdout)
	}},
	"coupling": {[]int{4}, func(args, skipDirs []string, stdout, _ io.Writer) (int, error) {
		return 0, Coupling(args[1], args[2], args[3], skipDirs, stdout)
	}},
	"fingerprint": {[]int{2}, func(args, _ []string, stdout, _ io.Writer) (int, error) {
		lines, err := Fingerprint(args[1])
		if err != nil {
			return 0, err
		}
		for _, l := range lines {
			fmt.Fprintln(stdout, l)
		}
		return 0, nil
	}},
	"c4": {[]int{3}, func(args, _ []string, stdout, _ io.Writer) (int, error) {
		return 0, C4(args[1], args[2], stdout)
	}},
	"check-c4": {[]int{4}, func(args, _ []string, stdout, _ io.Writer) (int, error) {
		ok, err := CheckC4(args[1], args[2], args[3], stdout)
		if err == nil && !ok {
			return 1, nil
		}
		return 0, err
	}},
}

// RunModelDir is kit code-graph model-dir: it prints ModelDir for args[0]
// under base, the architecture_dir setting.
func RunModelDir(base, home string, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	dir, err := ModelDir(base, home, args[0])
	if err != nil {
		fmt.Fprintf(stderr, "code-graph model-dir: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, dir)
	return 0
}

// readLines is file's lines.
func readLines(file string) ([]string, error) {
	f, err := os.Open(filepath.Clean(file))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// readTSV is file's lines holding a tab, each cut into at most n fields.
func readTSV(file string, n int) ([][]string, error) {
	lines, err := readLines(file)
	var rows [][]string
	for _, line := range lines {
		if strings.Contains(line, "\t") {
			rows = append(rows, strings.SplitN(line, "\t", n))
		}
	}
	return rows, err
}
