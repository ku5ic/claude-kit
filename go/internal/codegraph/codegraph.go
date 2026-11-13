package codegraph

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const usage = `usage: kit code-graph imports <root> > imports.tsv
       kit code-graph deps <root> <imports.tsv> <out dir> [split dir]
       kit code-graph coupling <git log> <imports-resolved.tsv> > coupling.tsv`

// Run is kit code-graph: its first argument names the report.
func Run(args []string, stdout, stderr io.Writer) int {
	var err error
	switch {
	case len(args) == 2 && args[0] == "imports":
		err = Imports(args[1], stdout)
	case (len(args) == 4 || len(args) == 5) && args[0] == "deps":
		split := ""
		if len(args) == 5 {
			split = args[4]
		}
		err = Deps(args[1], args[2], args[3], split, stdout)
	case len(args) == 3 && args[0] == "coupling":
		err = Coupling(args[1], args[2], stdout)
	default:
		fmt.Fprintln(stderr, usage)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "code-graph %s: %v\n", args[0], err)
		return 1
	}
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
