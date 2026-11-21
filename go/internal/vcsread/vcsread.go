// Package vcsread is kit read, which runs a git or code-host call only
// when it can't write.
package vcsread

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/proc"
	"github.com/ku5ic/claude-kit/go/internal/resolve/kittool"
)

// gitSubs are the git subcommands kit read runs. Bash permission rules
// can't hold git to reading, since git grep -O runs a program.
var gitSubs = []string{"log", "show", "blame", "tag", "rev-parse", "ls-files"}

// gitRunFlags write a file or run a program the repo's config names.
var gitRunFlags = []string{"--output", "--ext-diff", "--textconv"}

// gitQuiet turns off the programs git log -p and git show run by default:
// a textconv filter, and an external diff driver.
var gitQuiet = []string{"--no-ext-diff", "--no-textconv"}

// hostGroups are the gh and glab command groups kit read runs with view or
// list. api runs too, with no hostWrites flag. A group outside these could
// be an extension, a program of its own.
var hostGroups = map[string][]string{
	"gh":   {"pr", "issue", "release", "repo", "run", "workflow"},
	"glab": {"mr", "issue", "release", "repo"},
}

// hostWrites are the gh and glab flags that send a method or a body, or
// open a browser.
var hostWrites = []string{"-X", "--method", "-f", "--raw-field", "-F", "--field", "--input", "-w", "--web"}

// hostTimeout bounds a gh or glab call, as git.Timeout bounds git.
const hostTimeout = 30 * time.Second

// check is why argv can't run, nil when it only reads.
func check(argv []string) error {
	if len(argv) < 2 {
		return errors.New("name the tool and its command: git, gh, or glab")
	}
	tool, sub, args := argv[0], argv[1], argv[2:]
	switch tool {
	case "git":
		return checkGit(sub, args)
	case "gh", "glab":
		return checkHost(tool, sub, args)
	}
	return fmt.Errorf("%s isn't git, gh, or glab", tool)
}

func checkGit(sub string, args []string) error {
	if !slices.Contains(gitSubs, sub) {
		return fmt.Errorf("git %s isn't one of git %s", sub, strings.Join(gitSubs, ", "))
	}
	for _, a := range args {
		flag, _, _ := strings.Cut(a, "=")
		if strings.HasPrefix(a, "-O") || slices.Contains(gitRunFlags, flag) {
			return fmt.Errorf("git %s %s writes a file or runs a program", sub, a)
		}
		// git takes an abbreviation, so --no-l is --no-list, which leaves
		// list mode for a tag that's created.
		if sub == "tag" && strings.HasPrefix(a, "--no-l") {
			return fmt.Errorf("git tag %s leaves list mode", a)
		}
	}
	// In list mode git refuses -a, -d, and the other writing flags.
	if sub == "tag" && !slices.Contains(args, "-l") && !slices.Contains(args, "--list") {
		return errors.New("git tag runs only with -l or --list")
	}
	return nil
}

func checkHost(tool, group string, args []string) error {
	if group != "api" && (!slices.Contains(hostGroups[tool], group) || len(args) == 0 || args[0] != "view" && args[0] != "list") {
		return fmt.Errorf("%s runs only api, or view or list on %s", tool, strings.Join(hostGroups[tool], ", "))
	}
	for _, a := range args {
		// -XPOST and -iX POST hide the method in a cluster.
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && len(a) > 2 {
			return fmt.Errorf("%s: give each short flag alone, its value as the next word", a)
		}
		if flag, _, _ := strings.Cut(a, "="); slices.Contains(hostWrites, flag) {
			return fmt.Errorf("%s %s sends a write or opens a browser", tool, flag)
		}
	}
	return nil
}

// Run executes kit read's behavior and returns its exit status: the tool's
// own, or 2 for a call it refuses.
func Run(argv []string, stdout, stderr io.Writer) int {
	if err := check(argv); err != nil {
		fmt.Fprintln(stderr, "kit read:", err)
		return 2
	}
	args := argv[1:]
	if argv[0] == "git" && (args[0] == "log" || args[0] == "show") {
		args = slices.Concat(args[:1], gitQuiet, args[1:])
	}
	cmd := git.Command("", args...)
	if argv[0] != "git" {
		bin, err := kittool.Path(argv[0])
		if err != nil {
			fmt.Fprintln(stderr, "kit read:", err)
			return 1
		}
		cmd = proc.Command(hostTimeout, bin, argv[1:]...)
	}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return exit.ExitCode()
	}
	if err != nil {
		fmt.Fprintln(stderr, "kit read:", err)
		return 1
	}
	return 0
}
