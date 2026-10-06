package bashguard

import (
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/guard"
)

// command is one command of a pipeline, past its assignments and wrappers.
type command struct {
	st     *state
	seg    Segment
	call   int
	idx    int    // index of the command name in the call's words
	name   string // basename, quotes removed: "rm", r''m, /bin/rm all read rm
	args   []Word
	redirs []Redir
	inputs []string // < redirect sources
	rest   string   // normalized source after the name, to the pipeline's end
	text   string   // name + rest: what the per-command regexes read
	alone  bool     // the only command of its pipeline
}

func (c *command) block(reason, rule string) error { return c.st.h.Block(reason, rule) }

// readsSensitive blocks when any of paths is a credential file. nil when
// none is, or when sensitive-read is disabled, so the caller carries on.
func (c *command) readsSensitive(paths []string) error {
	for _, p := range paths {
		if guard.IsSensitive(c.st.cfg, p) {
			return c.block("reading a sensitive file is not permitted", "sensitive-read")
		}
	}
	return nil
}

func (c *command) values() []string {
	out := make([]string, len(c.args))
	for i, a := range c.args {
		out[i] = a.Value
	}
	return out
}

// operands are the non-option arguments: words starting with "-" are
// skipped until a "--", after which every word counts.
func (c *command) operands() []string {
	var out []string
	done := false
	for _, v := range c.values() {
		if !done {
			if v == "--" {
				done = true
				continue
			}
			if strings.HasPrefix(v, "-") {
				continue
			}
		}
		out = append(out, v)
	}
	return out
}

var (
	chmod777    = regexp.MustCompile(`chmod[[:space:]]+(-R[[:space:]]+)?777([[:space:]]|$)`)
	chmodSpace  = regexp.MustCompile(`chmod[[:space:]]`)
	chmodBroad  = regexp.MustCompile(`[[:space:]]["']?(\.|\.\.|/)["']?($|[[:space:]])`)
	chmodHome   = regexp.MustCompile(`[[:space:]]["']?(~|\$HOME|\$\{HOME\})["']?($|[[:space:]]|/)`)
	gitDiscard  = regexp.MustCompile(`[[:space:]](restore|checkout)[[:space:]]`)
	treeSpec    = regexp.MustCompile(`[[:space:]](\.|\*|--[[:space:]]?\.|:/)([[:space:]]|$)`)
	psqlCommand = regexp.MustCompile(`psql[[:space:]].*(-c|--command)[[:space:]]`)
	psqlDrop    = regexp.MustCompile(`(DROP[[:space:]]+(DATABASE|SCHEMA|TABLE)|TRUNCATE[[:space:]]+TABLE|DELETE[[:space:]]+FROM[[:space:]]+[a-zA-Z_]+[[:space:]]*;|DELETE[[:space:]]+FROM[[:space:]]+[a-zA-Z_]+[[:space:]]*$)`)
	redisBad    = regexp.MustCompile(`redis-cli[[:space:]].*(FLUSHALL|FLUSHDB|CONFIG[[:space:]]+SET|DEBUG[[:space:]]+SLEEP)`)
	awsS3Rm     = regexp.MustCompile(`aws[[:space:]]+([^[:space:]]+[[:space:]]+)*s3[[:space:]]+rm[[:space:]].*(--recursive)([[:space:]]|$)`)
	awsS3Rb     = regexp.MustCompile(`aws[[:space:]]+([^[:space:]]+[[:space:]]+)*s3[[:space:]]+rb[[:space:]].*(--force)([[:space:]]|$)`)
	awsEc2      = regexp.MustCompile(`aws[[:space:]]+([^[:space:]]+[[:space:]]+)*ec2[[:space:]]+terminate-instances`)
	gcloudDel   = regexp.MustCompile(`gcloud[[:space:]]+([^[:space:]]+[[:space:]]+)*delete([[:space:]]|$)`)
	kubectlDel  = regexp.MustCompile(`kubectl[[:space:]]+([^[:space:]]+[[:space:]]+)*delete([[:space:]]|$)`)
	tfDestroy   = regexp.MustCompile(`^[a-z]+[[:space:]]+([^[:space:]]+[[:space:]]+)*destroy([[:space:]]|$)`)
	tfAuto      = regexp.MustCompile(`^[a-z]+[[:space:]]+([^[:space:]]+[[:space:]]+)*apply[[:space:]].*(-auto-approve|--auto-approve)([[:space:]]|$)`)
	dockerPrune = regexp.MustCompile(`(system|volume|image|container|network)[[:space:]]+prune`)
	dockerAll   = regexp.MustCompile(`(^|[[:space:]])(-a|--all)([[:space:]]|$)|(^|[[:space:]])-[a-zA-Z]*a[a-zA-Z]*([[:space:]]|$)`)
	dockerForce = regexp.MustCompile(`(^|[[:space:]])(-f|--force)([[:space:]]|$)|(^|[[:space:]])-[a-zA-Z]*f[a-zA-Z]*([[:space:]]|$)`)
	findDelete  = regexp.MustCompile(`find[[:space:]].*-delete($|[[:space:]])`)
	findExecRm  = regexp.MustCompile(`find[[:space:]].*-exec[[:space:]]+rm([[:space:]]|$)`)
	keychainDel = regexp.MustCompile(`security[[:space:]]+delete-keychain`)
	pnpmInstall = regexp.MustCompile(`^(install|i)([[:space:]]|$)`)
	frozen      = regexp.MustCompile(`(^|[[:space:]])--frozen-lockfile([[:space:]]|$)`)
	npmGlobal   = regexp.MustCompile(`(npm|pnpm|yarn)[[:space:]]+(install|add|i)[[:space:]]+.*(-g|--global)`)
	yarnGlobal  = regexp.MustCompile(`yarn[[:space:]]+global[[:space:]]+add[[:space:]]`)
	bunGlobal   = regexp.MustCompile(`bun[[:space:]]+(add|install)[[:space:]]+.*(-g|--global)`)
	gitWriters  = regexp.MustCompile(` (checkout|restore|apply|mv|rm|stash|reset) `)
)

// readers only read the files they're given; any other command naming the
// overlay gets a prompt, since disabled_rules there switches guards off.
var readers = map[string]bool{
	"cat": true, "bat": true, "head": true, "tail": true, "less": true, "more": true, "jq": true, "rg": true,
	"grep": true, "diff": true, "wc": true, "ls": true, "stat": true, "file": true, "realpath": true,
	"readlink": true, "test": true, "[": true, "echo": true, "printf": true, "sed": true, "sd": true,
}

func (c *command) check() error {
	c.overlayWrite()
	// Any command prints what < feeds it: sort < .env reads it as well as cat.
	if err := c.readsSensitive(c.inputs); err != nil {
		return err
	}
	if err := c.rcWrite(); err != nil {
		return err
	}
	switch c.name {
	case "cd":
		// Tracked so a later command checks the right repo; a cd inside a
		// pipeline runs in a subshell and moves nothing.
		if !c.alone {
			return nil
		}
		prev := c.st.cwd
		switch {
		case len(c.args) == 0:
			c.st.cwd = c.st.home
		case c.args[0].Value == "-":
			if c.st.prev == "" {
				return nil
			}
			c.st.cwd = c.st.prev
		case strings.HasPrefix(c.args[0].Value, "-"):
			return nil
		default:
			c.st.cwd = resolveDir(c.st.home, c.st.cwd, c.args[0].Value)
		}
		c.st.prev = prev
	case "rm":
		return c.rm()
	case "dd", "shred", "wipefs", "mkfs":
		return c.block("low level disk or filesystem tool", "disk-tool")
	case "chmod":
		return c.chmod()
	case "git":
		return c.git()
	case "psql":
		if psqlCommand.MatchString(c.text) && psqlDrop.MatchString(c.text) {
			return c.block("destructive SQL via psql -c", "psql-destructive")
		}
	case "redis-cli":
		if redisBad.MatchString(c.text) {
			return c.block("destructive redis-cli command", "redis-destructive")
		}
	case "aws":
		for _, r := range []struct {
			re           *regexp.Regexp
			reason, rule string
		}{
			{awsS3Rm, "aws s3 rm --recursive deletes an entire bucket prefix", "aws-s3-recursive-rm"},
			{awsS3Rb, "aws s3 rb --force force-deletes a bucket and its contents", "aws-s3-force-rb"},
			{awsEc2, "aws ec2 terminate-instances is irreversible", "aws-ec2-terminate"},
		} {
			if r.re.MatchString(c.text) {
				if err := c.block(r.reason, r.rule); err != nil {
					return err
				}
			}
		}
	case "gcloud":
		if gcloudDel.MatchString(c.text) {
			return c.block("gcloud delete operation", "gcloud-delete")
		}
	case "kubectl":
		if kubectlDel.MatchString(c.text) {
			return c.block("kubectl delete", "kubectl-delete")
		}
	case "terraform", "tofu":
		// OpenTofu shares Terraform's CLI; the same slugs cover both.
		if tfDestroy.MatchString(c.text) {
			if err := c.block(c.name+" destroy", "terraform-destroy"); err != nil {
				return err
			}
		}
		if tfAuto.MatchString(c.text) {
			return c.block(c.name+" apply -auto-approve skips the plan review step", "terraform-auto-approve")
		}
	case "docker":
		// -a/-f are the only short flags these prune subcommands define, so
		// any cluster holding both letters is --all --force.
		if dockerPrune.MatchString(c.text) && dockerAll.MatchString(c.text) && dockerForce.MatchString(c.text) {
			return c.block("docker prune with --all --force wipes all unused resources", "docker-prune-all-force")
		}
	case "find":
		if findDelete.MatchString(c.text) {
			if err := c.block("find -delete", "find-delete"); err != nil {
				return err
			}
		}
		if findExecRm.MatchString(c.text) {
			return c.block("find -exec rm", "find-exec-rm")
		}
	case "security":
		if keychainDel.MatchString(c.text) {
			return c.block("keychain deletion", "keychain-delete")
		}
	case "npm", "npx", "pnpm", "yarn", "bun", "bunx", "pip", "pip3", "poetry", "uv", "pipenv":
		return c.packageManager()
	case "curl":
		return c.curl()
	case "wget":
		return c.wget()
	case "cat", "bat", "head", "tail", "less", "more", "strings":
		return c.readsSensitive(c.operands())
	case "grep", "rg":
		// The first non-option argument is the pattern, not a path.
		ops := c.operands()
		if len(ops) > 0 {
			ops = ops[1:]
		}
		return c.readsSensitive(ops)
	case "sh", "bash", "zsh", "dash":
		// -c runs a command string that never surfaces as its own Bash tool
		// call, bypassing the allow list. Short-option clusters only.
		for _, v := range c.values() {
			if v == "--" || !strings.HasPrefix(v, "-") {
				break
			}
			if !strings.HasPrefix(v, "--") && strings.Contains(v, "c") {
				return c.block("interpreter -c wrapping bypasses the permission allow list; run the command directly as a Bash tool call", "interpreter-c-wrap")
			}
		}
	case "eval":
		return c.block("eval runs a command string that bypasses the permission allow list; run the command directly as a Bash tool call", "interpreter-c-wrap")
	case "git-base.sh":
		// An explicit ask: with no decision, a Bash(git-base.sh *) allow
		// rule would approve --output=<file> silently.
		if !gitBaseFlagsSafe(strings.Fields(c.rest)) {
			c.st.ask("git-base.sh passes this flag to git, which can write files or run programs; confirm it")
		}
	case "sed", "sd":
		// sed only with -i; sd is always in place when given a file.
		inPlace := c.name == "sd"
		for _, v := range c.values() {
			if v == "--" {
				break
			}
			if strings.HasPrefix(v, "-") && strings.Contains(v, "i") {
				inPlace = true
			}
		}
		if !inPlace {
			return nil
		}
		for _, p := range c.operands() {
			if guard.IsRCFile(c.st.cfg, p) {
				if err := c.block("in-place edit of a shell rc file. Use the dotfiles repo.", "rc-inplace-edit"); err != nil {
					return err
				}
			}
			if c.st.isOverlayArg(p) {
				c.st.ask(overlayAsk)
			}
		}
	default:
		if strings.HasPrefix(c.name, "mkfs.") {
			return c.block("low level disk or filesystem tool", "disk-tool")
		}
	}
	return nil
}

// rcWrite blocks tee into a shell rc file and cp, mv, or install onto one;
// ln is left alone, as symlinking the dotfiles copy in is the fix.
func (c *command) rcWrite() error {
	var targets []string
	switch c.name {
	case "tee":
		targets = c.operands()
	case "cp", "mv", "install":
		if ops := c.operands(); len(ops) > 1 {
			targets = ops[len(ops)-1:]
		}
	}
	for _, p := range targets {
		if guard.IsRCFile(c.st.cfg, p) {
			return c.block("direct write to a shell rc file. Use the dotfiles repo.", "rc-redirect")
		}
	}
	return nil
}

// overlayWrite asks when a command that isn't a pure reader names the
// overlay: tee, cp, mv, install, ln all write it. yq writes only with -i,
// git only through checkout/restore/apply/mv/rm/stash/reset.
func (c *command) overlayWrite() {
	all := append([]string{c.name}, c.values()...)
	switch {
	case c.name == "yq":
		if !slices.ContainsFunc(c.values(), func(v string) bool { return strings.HasPrefix(v, "-i") || strings.HasPrefix(v, "--inplace") }) {
			return
		}
	case c.name == "git":
		if !gitWriters.MatchString(" " + strings.Join(all, " ") + " ") {
			return
		}
	case readers[c.name]:
		return
	}
	for _, v := range c.values() {
		_, after, hasEq := strings.Cut(v, "=")
		if c.st.isOverlayArg(v) || (hasEq && c.st.isOverlayArg(after)) {
			c.st.ask(overlayAsk)
			return
		}
	}
}

func (c *command) rm() error {
	// Whole words only: rm -rf *.log and rm -rf dist/* stay allowed.
	force, broad := false, false
	for _, v := range c.values() {
		switch {
		case v == "--recursive" || v == "--force":
			force = true
		case strings.HasPrefix(v, "--"):
		case strings.HasPrefix(v, "-") && strings.ContainsAny(v, "rRfF"):
			force = true
		case slices.Contains([]string{"/", "/*", "~", "~/", "~/*", "$HOME", "${HOME}", "$HOME/", "${HOME}/", "$HOME/*", "${HOME}/*", ".", "..", "./", "../", "*"}, v):
			broad = true
		}
	}
	if force && broad {
		return c.block("rm with recursive force against root, home, or cwd", "rm-recursive")
	}
	return nil
}

func (c *command) chmod() error {
	if chmod777.MatchString(c.text) {
		if err := c.block("chmod 777", "chmod-777"); err != nil {
			return err
		}
	}
	if chmodSpace.MatchString(c.text) && strings.Contains(c.text, "+x") &&
		(chmodBroad.MatchString(c.text) || chmodHome.MatchString(c.text)) {
		return c.block("broad chmod +x against root, home, or cwd", "chmod-broad-x")
	}
	return nil
}

// git splits past git's global options: the subcommand, its arguments, and
// the -C directory.
func (c *command) git() error {
	words := c.values()
	sub, dir := "", ""
	var args []string
	for i := 0; i < len(words); {
		w := words[i]
		switch {
		case w == "-C":
			if i+1 < len(words) {
				dir = words[i+1]
			}
			i += 2
			continue
		case slices.Contains([]string{"-c", "--git-dir", "--work-tree", "--namespace", "--config-env", "--super-prefix"}, w):
			i += 2
			continue
		case strings.HasPrefix(w, "-"):
			i++
			continue
		}
		sub, args = w, words[i+1:]
		break
	}
	has := func(arg string) bool { return slices.Contains(args, arg) }

	switch sub {
	case "commit", "push", "merge", "rebase":
		if has("--no-verify") {
			if err := c.block("use of --no-verify bypasses pre-commit and pre-push hooks", "git-no-verify"); err != nil {
				return err
			}
		}
	}
	switch sub {
	case "push":
		if err := c.gitPush(args, dir); err != nil {
			return err
		}
	case "commit":
		if err := c.gitCommit(args); err != nil {
			return err
		}
	case "reset":
		if has("--hard") {
			for _, ref := range nonOptions(args) {
				if c.st.isProtected(ref) {
					if err := c.block("git reset --hard on protected branch", "git-reset-hard"); err != nil {
						return err
					}
				}
			}
		}
	case "config":
		if has("--global") {
			if err := c.block("git config --global from a project session", "git-config-global"); err != nil {
				return err
			}
		}
	}
	// Tree-wide pathspecs only: bare dot, -- ., :/, or a bare star.
	// --staged without --worktree is allowed: unstaging isn't destructive.
	if gitDiscard.MatchString(c.text) && treeSpec.MatchString(c.text) &&
		(!strings.Contains(c.text, "--staged") || strings.Contains(c.text, "--worktree")) {
		return c.block("tree-wide discard of working-tree changes; restore individual files explicitly", "git-tree-discard")
	}
	return nil
}

func nonOptions(words []string) []string {
	var out []string
	done := false
	for _, w := range words {
		if !done && w == "--" {
			done = true
			continue
		}
		if !done && strings.HasPrefix(w, "-") {
			continue
		}
		out = append(out, w)
	}
	return out
}

func (c *command) gitPush(args []string, dir string) error {
	wantValue, optsDone, haveRemote, tagsOnly := false, false, false, false
	var refspecs []string
	for _, a := range args {
		if wantValue {
			wantValue = false
			continue
		}
		if !optsDone {
			switch {
			case a == "--":
				optsDone = true
				continue
			case a == "--force":
				if err := c.block("git push --force. Use --force-with-lease if you must.", "git-force-push"); err != nil {
					return err
				}
				continue
			case a == "--mirror":
				if err := c.block("git push --mirror overwrites every remote ref, protected branches included", "git-force-push"); err != nil {
					return err
				}
				continue
			case a == "--tags":
				// Pushes tags, not the current branch.
				tagsOnly = true
				continue
			case slices.Contains([]string{"--repo", "--push-option", "--receive-pack", "--exec"}, a):
				wantValue = true
				continue
			case strings.HasPrefix(a, "--"):
				continue
			case strings.HasPrefix(a, "-"):
				if strings.Contains(a, "f") {
					if err := c.block("git push -f. Use --force-with-lease if you must.", "git-force-push"); err != nil {
						return err
					}
				}
				if strings.HasSuffix(a, "o") {
					wantValue = true
				}
				continue
			}
		}
		if !haveRemote {
			haveRemote = true
			continue
		}
		refspecs = append(refspecs, a)
	}
	for _, ref := range refspecs {
		if strings.HasPrefix(ref, "+") {
			if err := c.block("force push via a +refspec. Use --force-with-lease if you must.", "git-force-push"); err != nil {
				return err
			}
		}
		dst := ref[strings.LastIndexByte(ref, ':')+1:]
		if dst == "HEAD" {
			dst = c.st.currentBranch(dir)
		}
		if c.st.isProtected(dst) {
			if err := c.block("push to a protected branch; use a feature branch", "git-push-protected"); err != nil {
				return err
			}
		}
	}
	if len(refspecs) == 0 && !tagsOnly && c.st.isProtected(c.st.currentBranch(dir)) {
		if err := c.block("push to a protected branch; use a feature branch", "git-push-protected"); err != nil {
			return err
		}
	}
	c.st.ask("git push publishes commits to a remote; confirm the destination")
	return nil
}

// gitCommit treats -n as --no-verify. Short clusters are scanned up to the
// first option that takes a value: in -mn the n is the message.
func (c *command) gitCommit(args []string) error {
	valued := []string{"--message", "--file", "--author", "--date", "--template", "--trailer", "--cleanup",
		"--reuse-message", "--reedit-message", "--fixup", "--squash", "--pathspec-from-file"}
	wantValue := false
	for _, a := range args {
		if wantValue {
			wantValue = false
			continue
		}
		switch {
		case a == "--":
			return nil
		case slices.Contains(valued, a):
			wantValue = true
		case strings.HasPrefix(a, "--"):
		case strings.HasPrefix(a, "-"):
		cluster:
			for i := 1; i < len(a); i++ {
				switch a[i] {
				case 'n':
					if err := c.block("git commit -n bypasses pre-commit hooks, same as --no-verify", "git-no-verify"); err != nil {
						return err
					}
					break cluster
				case 'm', 'F', 'C', 'c', 't':
					wantValue = i == len(a)-1
					break cluster
				case 'u', 'S':
					// Optional values, only ever attached: -uno, -S<keyid>.
					break cluster
				}
			}
		}
	}
	return nil
}
