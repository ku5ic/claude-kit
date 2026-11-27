package bashguard

import (
	"path"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/layout"
)

// leaseInstead is a force push's fix, the second line of its reason.
var leaseInstead = "\n" + layout.KV("do", "use --force-with-lease if you must")

// git checks a git command: --no-verify on any subcommand that runs hooks,
// the subcommand's own check from gitChecks, and a tree-wide discard.
func (c *command) git() error {
	words := c.values()
	sub, args, dir := classify.GitSubcommand(words)
	switch sub {
	case "commit", "push", "merge", "rebase":
		// git takes any unambiguous prefix: --no-verif is --no-verify.
		if slices.ContainsFunc(args, func(a string) bool { return len(a) > len("--no-") && abbrevOf(a, "--no-verify") }) {
			if err := c.block("use of --no-verify bypasses pre-commit and pre-push hooks", "git-no-verify"); err != nil {
				return err
			}
		}
		exported, bare := c.st.setVars()
		switch {
		case hooksOff(words[:max(0, len(words)-len(args)-1)]) || hooksOffByEnv(append(slices.Clone(c.assigns), exported...)):
			if err := c.block("core.hooksPath set on the command line skips the repo's hooks, as --no-verify does", "git-no-verify"); err != nil {
				return err
			}
		case hooksOffByEnv(bare):
			c.st.ask("a shell variable sets core.hooksPath, which skips the repo's hooks if the variable is already exported; confirm", "git-no-verify")
		}
	}
	if reason := historyLoss(sub, args); reason != "" {
		c.st.ask(reason, "git-history-loss")
	}
	if check, ok := gitChecks[sub]; ok {
		if err := check(c, args, dir); err != nil {
			return err
		}
	}
	if treeDiscard(sub, args) {
		return c.block("tree-wide discard of working-tree changes\n"+layout.KV("do", "restore individual files explicitly"), "git-tree-discard")
	}
	return nil
}

// hooksOff is true when git's own options, the words before its
// subcommand, point core.hooksPath anywhere: -c core.hooksPath=x or
// --config-env=core.hooksPath=VAR. A config key's case doesn't matter.
func hooksOff(globals []string) bool {
	for i, g := range globals {
		var setting string
		switch {
		case (g == "-c" || g == "--config-env") && i+1 < len(globals):
			setting = globals[i+1]
		case strings.HasPrefix(g, "--config-env="):
			setting = strings.TrimPrefix(g, "--config-env=")
		case strings.HasPrefix(g, "-c"):
			setting = g[2:]
		}
		if key, _, ok := strings.Cut(setting, "="); ok && strings.EqualFold(key, "core.hooksPath") {
			return true
		}
	}
	return false
}

// hooksOffByEnv is true when assigns, the VAR=value words a git command runs
// with, its own and the command's earlier ones, point core.hooksPath anywhere, as -c does: a GIT_CONFIG_KEY_<n>
// naming it, or GIT_CONFIG_PARAMETERS holding it. A config key's case
// doesn't matter.
func hooksOffByEnv(assigns []string) bool {
	return slices.ContainsFunc(assigns, func(a string) bool {
		name, value, _ := strings.Cut(a, "=")
		switch {
		case strings.HasPrefix(name, "GIT_CONFIG_KEY_"):
			return strings.EqualFold(value, "core.hooksPath")
		case name == "GIT_CONFIG_PARAMETERS":
			return strings.Contains(strings.ToLower(value), "core.hookspath")
		}
		return false
	})
}

// historyLoss is the ask reason for a git command that drops history git
// can't bring back, "" for any other.
func historyLoss(sub string, args []string) string {
	first := ""
	if ops := nonOptions(args); len(ops) > 0 {
		first = ops[0]
	}
	has := func(prefixes ...string) bool {
		return slices.ContainsFunc(args, func(a string) bool {
			return slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(a, p) })
		})
	}
	switch {
	case sub == "stash" && first == "clear":
		return "git stash clear deletes every stash, and git keeps nothing to bring them back; confirm"
	case sub == "filter-branch":
		return "git filter-branch rewrites every commit it touches; confirm"
	case sub == "reflog" && (first == "delete" || first == "expire" && has("--expire=now", "--expire=all", "--expire-unreachable=now", "--expire-unreachable=all")):
		return "git reflog " + first + " drops the entries that recover lost commits; confirm"
	case sub == "gc" && has("--prune=now", "--prune=all"):
		return "git gc --prune=now deletes unreachable commits at once, with no grace period; confirm"
	case sub == "checkout" && has("-f", "--force"):
		return "git checkout --force discards uncommitted changes in the files it switches; confirm"
	}
	return ""
}

// treeDiscard is true for a git restore or checkout of a pathspec naming
// the whole tree. restore --staged without --worktree only unstages, which
// discards nothing.
func treeDiscard(sub string, args []string) bool {
	if sub != "restore" && sub != "checkout" {
		return false
	}
	// Git reads options after the pathspec too (restore . --staged), so
	// the verdict waits for every word.
	staged, worktree, operands, wide := false, false, false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case operands:
			wide = wide || treeWide(a)
		case a == "--":
			operands = true
		case a == "--staged" || a == "-S":
			staged = true
		case a == "--worktree" || a == "-W":
			worktree = true
		case slices.Contains([]string{"-s", "--source", "-b", "-B", "--orphan", "--pathspec-from-file"}, a):
			i++ // the option's value
		case strings.HasPrefix(a, "-"):
		case treeWide(a):
			wide = true
		}
	}
	return wide && (!staged || worktree)
}

// treeWide is true for a pathspec naming the whole tree: ., ./, *, :/, :/:,
// :(top), a magic pathspec whose path does, and an exclusion (:!docs,
// :(exclude)docs), which names everything else.
func treeWide(spec string) bool {
	switch {
	case strings.HasPrefix(spec, ":!"), strings.HasPrefix(spec, ":^"):
		return true
	case strings.HasPrefix(spec, ":("):
		end := strings.IndexByte(spec, ')')
		if end < 0 {
			return false
		}
		if slices.Contains(strings.Split(spec[2:end], ","), "exclude") {
			return true
		}
		spec = spec[end+1:]
	case strings.HasPrefix(spec, ":/"):
		spec = strings.TrimPrefix(spec[2:], ":")
	}
	p := path.Clean(spec)
	return p == "." || strings.Trim(p, "*/") == ""
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
	"add":    (*command).gitAdd,
}

// gitAdd asks before staging a credential file: a commit carries it into
// history, where taking it out means a rewrite and a key rotation.
func (c *command) gitAdd(args []string, _ string) error {
	if slices.ContainsFunc(nonOptions(args), func(p string) bool {
		return guard.IsSensitive(c.st.cfg, c.st.home, c.st.abs(unquote(p)))
	}) {
		c.st.ask("git add stages a credential file, and a commit would carry it into history; confirm it belongs in the repo", "git-add-credential")
	}
	return nil
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
	c.st.ask("git reset --hard discards uncommitted changes; confirm", "git-reset-hard")
	return nil
}

func (c *command) gitClean(args []string, _ string) error {
	dryRun := slices.Contains(args, "--dry-run") || slices.ContainsFunc(args, func(a string) bool { return shortCluster(a, "n") })
	if !dryRun {
		c.st.ask("git clean deletes untracked files; confirm, or run it with -n first", "git-clean")
	}
	return nil
}

// gitDelete asks before git branch or git tag deletes one.
func (c *command) gitDelete(sub string, args []string) error {
	if slices.ContainsFunc(args, func(a string) bool { return a == "--delete" || shortCluster(a, "dD") }) {
		c.st.ask("git "+sub+" deletion; confirm the name", "git-ref-delete")
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
		case abbrevOf(a, "--force"):
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
	c.st.ask("git push publishes commits to a remote; confirm the destination", "git-push")
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
