// Package gitbase finds the base ref of the current checkout, as
// kit git-base: an explicit ref, else the upstream (unless it is only this
// branch's own push target), else origin/HEAD, else main, master, develop,
// or trunk.
package gitbase

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/git"
)

// Mode is what to print for the base: the ref, the diff against it, or the
// log since it.
type Mode int

const (
	Base Mode = iota
	Diff
	Log
)

// Args are kit git-base's arguments, parsed.
type Args struct {
	Mode     Mode
	Explicit string   // first word that resolves as a ref
	Extra    []string // flags, and a flag's value, passed through to git
	Paths    []string // pathspecs after --
}

// parse splits kit git-base's arguments. The mode is a flag (--diff, --log)
// so a branch named diff or log still works as the base. The base is the
// first word that resolves as a ref; a word right after a flag is that
// flag's value (-n 5); any other word is an error, not a silent fallback
// that would diff against the wrong branch.
func parse(argv []string) (Args, error) {
	var a Args
	if len(argv) > 0 {
		switch argv[0] {
		case "--diff":
			a.Mode, argv = Diff, argv[1:]
		case "--log":
			a.Mode, argv = Log, argv[1:]
		}
	}
	inPaths := false
	prev := ""
	for _, arg := range argv {
		switch {
		case inPaths:
			a.Paths = append(a.Paths, arg)
		case arg == "--":
			inPaths = true
		case strings.HasPrefix(arg, "-"):
			a.Extra = append(a.Extra, arg)
		case a.Explicit == "" && verifyIn("", arg):
			a.Explicit = arg
		case strings.HasPrefix(prev, "-") && !strings.Contains(prev, "="):
			a.Extra = append(a.Extra, arg)
		default:
			return a, fmt.Errorf("kit git-base: '%s' is not a ref", arg)
		}
		prev = arg
	}
	return a, nil
}

func verifyIn(dir, ref string) bool {
	_, err := git.Line(dir, "rev-parse", "--verify", "--quiet", ref)
	return err == nil
}

// Resolve returns the base ref for the repository at dir ("" is the
// working directory), or false when nothing resolves.
func Resolve(dir, explicit string) (string, bool) {
	run := func(args ...string) (string, error) { return git.Line(dir, args...) }
	verify := func(ref string) bool { return verifyIn(dir, ref) }
	if explicit != "" && verify(explicit) {
		return explicit, true
	}
	if upstream, err := run("rev-parse", "--abbrev-ref", "@{upstream}"); err == nil {
		current, _ := run("rev-parse", "--abbrev-ref", "HEAD")
		// ${upstream#*/}: drop the remote name; no slash leaves it whole.
		branch := upstream
		if _, rest, found := strings.Cut(upstream, "/"); found {
			branch = rest
		}
		if branch != current {
			return upstream, true
		}
	}
	if _, err := run("symbolic-ref", "refs/remotes/origin/HEAD"); err == nil {
		if resolved, err := run("symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil && verify(resolved) {
			return resolved, true
		}
	}
	for _, b := range []string{"main", "master", "develop", "trunk"} {
		if verify(b) {
			return b, true
		}
	}
	return "", false
}

// Run executes kit git-base's behavior and returns its exit status.
func Run(argv []string, stdout, stderr io.Writer) int {
	a, err := parse(argv)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	base, ok := Resolve("", a.Explicit)
	if !ok {
		return 1
	}
	var args []string
	switch a.Mode {
	case Base:
		fmt.Fprintln(stdout, base)
		return 0
	case Diff:
		args = append(append([]string{"diff"}, a.Extra...), base+"...HEAD", "--")
	case Log:
		args = append(append([]string{"log", "--oneline"}, a.Extra...), base+"..HEAD", "--")
	}
	cmd := git.Command("", append(args, a.Paths...)...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = stdout, stderr, os.Stdin
	if err := cmd.Run(); err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			return exit.ExitCode()
		}
		fmt.Fprintln(stderr, "kit git-base:", err)
		return 1
	}
	return 0
}
