package bashguard

import (
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/layout"
)

var (
	gitDiscard = regexp.MustCompile(`[[:space:]](restore|checkout)[[:space:]]`)
	treeSpec   = regexp.MustCompile(`[[:space:]](\.|\*|--[[:space:]]?\.|:/)([[:space:]]|$)`)
)

// leaseInstead is a force push's fix, the second line of its reason.
var leaseInstead = "\n" + layout.KV("do", "use --force-with-lease if you must")

// git checks a git command: --no-verify on any subcommand that runs hooks,
// the subcommand's own check from gitChecks, and a tree-wide discard.
func (c *command) git() error {
	sub, args, dir := classify.GitSubcommand(c.values())
	switch sub {
	case "commit", "push", "merge", "rebase":
		if slices.Contains(args, "--no-verify") {
			if err := c.block("use of --no-verify bypasses pre-commit and pre-push hooks", "git-no-verify"); err != nil {
				return err
			}
		}
	}
	if check, ok := gitChecks[sub]; ok {
		if err := check(c, args, dir); err != nil {
			return err
		}
	}
	// Tree-wide pathspecs only: bare dot, -- ., :/, or a bare star.
	// --staged without --worktree is allowed: unstaging isn't destructive.
	if gitDiscard.MatchString(c.text) && treeSpec.MatchString(c.text) &&
		(!strings.Contains(c.text, "--staged") || strings.Contains(c.text, "--worktree")) {
		return c.block("tree-wide discard of working-tree changes\n"+layout.KV("do", "restore individual files explicitly"), "git-tree-discard")
	}
	return nil
}

// gitChecks are each git subcommand's own check, given its args and the
// -C dir.
var gitChecks = map[string]func(c *command, args []string, dir string) error{
	"push":   (*command).gitPush,
	"commit": (*command).gitCommit,
	"reset":  (*command).gitReset,
	"clean":  (*command).gitClean,
	"branch": func(c *command, args []string, _ string) error { return c.gitDelete("branch", args) },
	"tag":    func(c *command, args []string, _ string) error { return c.gitDelete("tag", args) },
	"config": (*command).gitConfig,
}

func (c *command) gitReset(args []string, _ string) error {
	if !slices.Contains(args, "--hard") {
		return nil
	}
	for _, ref := range nonOptions(args) {
		if c.st.isProtected(ref) {
			if err := c.block("git reset --hard on protected branch", "git-reset-hard"); err != nil {
				return err
			}
		}
	}
	c.st.ask("git reset --hard discards uncommitted changes; confirm")
	return nil
}

func (c *command) gitClean(args []string, _ string) error {
	dryRun := slices.Contains(args, "--dry-run") || slices.ContainsFunc(args, func(a string) bool { return shortCluster(a, "n") })
	if !dryRun {
		c.st.ask("git clean deletes untracked files; confirm, or run it with -n first")
	}
	return nil
}

// gitDelete asks before git branch or git tag deletes one.
func (c *command) gitDelete(sub string, args []string) error {
	if slices.ContainsFunc(args, func(a string) bool { return a == "--delete" || shortCluster(a, "dD") }) {
		c.st.ask("git " + sub + " deletion; confirm the name")
	}
	return nil
}

func (c *command) gitConfig(args []string, _ string) error {
	if slices.Contains(args, "--global") {
		return c.block("git config --global from a project session", "git-config-global")
	}
	return nil
}

// push is git push's arguments as the guard reads them.
type push struct {
	forced   []string // block reasons for --force, -f, --mirror, in order
	tagsOnly bool     // --tags: pushes tags, not the current branch
	refspecs []string
}

func parsePush(args []string) push {
	var p push
	wantValue, optsDone, haveRemote := false, false, false
	for _, a := range args {
		switch {
		case wantValue:
			wantValue = false
		case optsDone || !strings.HasPrefix(a, "-"):
			if haveRemote {
				p.refspecs = append(p.refspecs, a)
			}
			haveRemote = true
		case a == "--":
			optsDone = true
		case a == "--force":
			p.forced = append(p.forced, "git push --force"+leaseInstead)
		case a == "--mirror":
			p.forced = append(p.forced, "git push --mirror overwrites every remote ref, protected branches included")
		case a == "--tags":
			p.tagsOnly = true
		case slices.Contains([]string{"--repo", "--push-option", "--receive-pack", "--exec"}, a):
			wantValue = true
		case strings.HasPrefix(a, "--"):
		default:
			if strings.Contains(a, "f") {
				p.forced = append(p.forced, "git push -f"+leaseInstead)
			}
			wantValue = strings.HasSuffix(a, "o")
		}
	}
	for _, ref := range p.refspecs {
		if strings.HasPrefix(ref, "+") {
			p.forced = append(p.forced, "force push via a +refspec"+leaseInstead)
		}
	}
	return p
}

func (c *command) gitPush(args []string, dir string) error {
	p := parsePush(args)
	for _, reason := range p.forced {
		if err := c.block(reason, "git-force-push"); err != nil {
			return err
		}
	}
	dsts := make([]string, 0, len(p.refspecs))
	for _, ref := range p.refspecs {
		dst := ref[strings.LastIndexByte(ref, ':')+1:]
		if dst == "HEAD" {
			dst = c.st.currentBranch(dir)
		}
		dsts = append(dsts, dst)
	}
	if len(p.refspecs) == 0 && !p.tagsOnly {
		dsts = append(dsts, c.st.currentBranch(dir))
	}
	for _, dst := range dsts {
		if c.st.isProtected(dst) {
			if err := c.block("push to a protected branch\n"+layout.KV("do", "push a feature branch"), "git-push-protected"); err != nil {
				return err
			}
		}
	}
	c.st.ask("git push publishes commits to a remote; confirm the destination")
	return nil
}

// gitCommit treats -n as --no-verify.
func (c *command) gitCommit(args []string, _ string) error {
	noVerify := false
	commitOptions(args, func(opt, _ string) { noVerify = noVerify || opt == "-n" })
	if noVerify {
		return c.block("git commit -n bypasses pre-commit hooks, same as --no-verify", "git-no-verify")
	}
	return nil
}

// commitOptions walks git commit's args as git reads them, up to "--",
// calling fn with each option and its value ("" for none): a long one by
// name, a short cluster's letters one by one, up to the first that takes a
// value (in -mn the n is the message).
func commitOptions(args []string, fn func(opt, value string)) {
	valued := []string{"--message", "--file", "--author", "--date", "--template", "--trailer", "--cleanup",
		"--reuse-message", "--reedit-message", "--fixup", "--squash", "--pathspec-from-file"}
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() string {
			if i+1 < len(args) {
				i++
				return args[i]
			}
			return ""
		}
		switch {
		case a == "--":
			return
		case slices.Contains(valued, a):
			fn(a, next())
		case strings.HasPrefix(a, "--"):
			name, value, _ := strings.Cut(a, "=")
			fn(name, value)
		case strings.HasPrefix(a, "-"):
			for j := 1; j < len(a); j++ {
				opt := "-" + a[j:j+1]
				switch a[j] {
				case 'm', 'F', 'C', 'c', 't':
					value := a[j+1:]
					if value == "" {
						value = next()
					}
					fn(opt, value)
					j = len(a)
				case 'u', 'S':
					// Optional values, only ever attached: -uno, -S<keyid>.
					fn(opt, a[j+1:])
					j = len(a)
				default:
					fn(opt, "")
				}
			}
		}
	}
}

// isProtected is true when ref names a protected branch, ignoring
// refs/heads/, refs/remotes/<remote>/, and origin/ prefixes.
func (st *state) isProtected(ref string) bool {
	ref = strings.TrimPrefix(ref, "refs/heads/")
	if rest, ok := strings.CutPrefix(ref, "refs/remotes/"); ok {
		if _, branch, found := strings.Cut(rest, "/"); found {
			ref = branch
		}
	}
	ref = strings.TrimPrefix(ref, "origin/")
	return slices.Contains(st.cfg.ProtectedBranches, ref)
}

// currentBranch of the repo a git command targets: -C resolved against the
// segment's cwd. Empty outside a repo or on a detached HEAD.
func (st *state) currentBranch(gitDir string) string {
	dir := st.cwd
	if gitDir != "" {
		dir = st.abs(gitDir)
	}
	branch, _ := git.Branch(dir)
	return branch
}
