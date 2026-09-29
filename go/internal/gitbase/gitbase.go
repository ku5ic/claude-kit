// Package gitbase finds the base ref of the current checkout, as
// git-base.sh: an explicit ref, else the upstream (unless it is only this
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
)

// Mode is what to print for the base: the ref, the diff against it, or the
// log since it.
type Mode int

const (
	Base Mode = iota
	Diff
	Log
)

// Args are git-base.sh's arguments, parsed.
type Args struct {
	Mode     Mode
	Explicit string   // first word that resolves as a ref
	Extra    []string // flags, and a flag's value, passed through to git
	Paths    []string // pathspecs after --
}

// Parse splits git-base.sh's arguments. The mode is a flag (--diff, --log)
// so a branch named diff or log still works as the base. The base is the
// first word that resolves as a ref; a word right after a flag is that
// flag's value (-n 5); any other word is an error, not a silent fallback
// that would diff against the wrong branch.
func Parse(argv []string) (Args, error) {
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
		case a.Explicit == "" && verify(arg):
			a.Explicit = arg
		case strings.HasPrefix(prev, "-") && !strings.Contains(prev, "="):
			a.Extra = append(a.Extra, arg)
		default:
			return a, fmt.Errorf("git-base.sh: '%s' is not a ref", arg)
		}
		prev = arg
	}
	return a, nil
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	return strings.TrimSpace(string(out)), err
}

func verify(ref string) bool {
	_, err := git("rev-parse", "--verify", "--quiet", ref)
	return err == nil
}

// Resolve returns the base ref, or false when nothing resolves.
func Resolve(explicit string) (string, bool) {
	if explicit != "" && verify(explicit) {
		return explicit, true
	}
	if upstream, err := git("rev-parse", "--abbrev-ref", "@{upstream}"); err == nil {
		current, _ := git("rev-parse", "--abbrev-ref", "HEAD")
		// ${upstream#*/}: drop the remote name; no slash leaves it whole.
		branch := upstream
		if _, rest, found := strings.Cut(upstream, "/"); found {
			branch = rest
		}
		if branch != current {
			return upstream, true
		}
	}
	if _, err := git("symbolic-ref", "refs/remotes/origin/HEAD"); err == nil {
		if resolved, err := git("symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil && verify(resolved) {
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

// Run executes git-base.sh's behavior and returns its exit status.
func Run(argv []string, stdout, stderr io.Writer) int {
	a, err := Parse(argv)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	base, ok := Resolve(a.Explicit)
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
	cmd := exec.Command("git", append(args, a.Paths...)...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = stdout, stderr, os.Stdin
	if err := cmd.Run(); err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			return exit.ExitCode()
		}
		fmt.Fprintln(stderr, "git-base.sh:", err)
		return 1
	}
	return 0
}
