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
	gitWriters  = regexp.MustCompile(` (checkout|restore|apply|mv|rm|stash|reset) `)
)

// readers only read the files they're given; any other command naming the
// overlay gets a prompt, since disabled_rules there switches guards off.
var readers = map[string]bool{
	"cat": true, "bat": true, "head": true, "tail": true, "less": true, "more": true, "jq": true, "rg": true,
	"grep": true, "diff": true, "wc": true, "ls": true, "stat": true, "file": true, "realpath": true,
	"readlink": true, "test": true, "[": true, "echo": true, "printf": true, "sed": true, "sd": true,
}

// destructive is the commands blocked by pattern: one of names, with every
// regex matching its text. %s in reason is the command's name.
var destructive = []struct {
	names        []string
	all          []*regexp.Regexp
	reason, rule string
}{
	{[]string{"psql"}, []*regexp.Regexp{psqlCommand, psqlDrop}, "destructive SQL via psql -c", "psql-destructive"},
	{[]string{"redis-cli"}, []*regexp.Regexp{redisBad}, "destructive redis-cli command", "redis-destructive"},
	{[]string{"aws"}, []*regexp.Regexp{awsS3Rm}, "aws s3 rm --recursive deletes an entire bucket prefix", "aws-s3-recursive-rm"},
	{[]string{"aws"}, []*regexp.Regexp{awsS3Rb}, "aws s3 rb --force force-deletes a bucket and its contents", "aws-s3-force-rb"},
	{[]string{"aws"}, []*regexp.Regexp{awsEc2}, "aws ec2 terminate-instances is irreversible", "aws-ec2-terminate"},
	{[]string{"gcloud"}, []*regexp.Regexp{gcloudDel}, "gcloud delete operation", "gcloud-delete"},
	{[]string{"kubectl"}, []*regexp.Regexp{kubectlDel}, "kubectl delete", "kubectl-delete"},
	// OpenTofu shares Terraform's CLI; the same slugs cover both.
	{[]string{"terraform", "tofu"}, []*regexp.Regexp{tfDestroy}, "%s destroy", "terraform-destroy"},
	{[]string{"terraform", "tofu"}, []*regexp.Regexp{tfAuto}, "%s apply -auto-approve skips the plan review step", "terraform-auto-approve"},
	// -a/-f are the only short flags these prune subcommands define, so any
	// cluster holding both letters is --all --force.
	{[]string{"docker"}, []*regexp.Regexp{dockerPrune, dockerAll, dockerForce}, "docker prune with --all --force wipes all unused resources", "docker-prune-all-force"},
	{[]string{"find"}, []*regexp.Regexp{findDelete}, "find -delete", "find-delete"},
	{[]string{"find"}, []*regexp.Regexp{findExecRm}, "find -exec rm", "find-exec-rm"},
	{[]string{"security"}, []*regexp.Regexp{keychainDel}, "keychain deletion", "keychain-delete"},
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
	for _, d := range destructive {
		if slices.Contains(d.names, c.name) && !slices.ContainsFunc(d.all, func(re *regexp.Regexp) bool { return !re.MatchString(c.text) }) {
			if err := c.block(strings.ReplaceAll(d.reason, "%s", c.name), d.rule); err != nil {
				return err
			}
		}
	}
	switch c.name {
	case "cd":
		c.cd()
	case "rm":
		return c.rm()
	case "dd", "shred", "wipefs", "mkfs":
		return c.block("low level disk or filesystem tool", "disk-tool")
	case "chmod":
		return c.chmod()
	case "git":
		return c.git()
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
		return c.interpreterC()
	case "eval":
		return c.block("eval runs a command string that bypasses the permission allow list; run the command directly as a Bash tool call", "interpreter-c-wrap")
	case "kit":
		// An explicit ask: with no decision, a Bash(kit git-base *) allow
		// rule would approve --output=<file> silently.
		if words := strings.Fields(c.rest); len(words) > 0 && words[0] == "git-base" && !gitBaseFlagsSafe(words[1:]) {
			c.st.ask("kit git-base passes this flag to git, which can write files or run programs; confirm it")
		}
	case "sed", "sd":
		return c.inPlaceEdit()
	default:
		if _, ok := c.st.cfg.Manager(c.name); ok {
			return c.packageManager()
		}
		if strings.HasPrefix(c.name, "mkfs.") {
			return c.block("low level disk or filesystem tool", "disk-tool")
		}
	}
	return nil
}

// cd tracks the directory a later command runs in, so it checks the right
// repo; a cd inside a pipeline runs in a subshell and moves nothing.
func (c *command) cd() {
	if !c.alone {
		return
	}
	prev := c.st.cwd
	switch {
	case len(c.args) == 0:
		c.st.cwd = c.st.home
	case c.args[0].Value == "-":
		if c.st.prev == "" {
			return
		}
		c.st.cwd = c.st.prev
	case strings.HasPrefix(c.args[0].Value, "-"):
		return
	default:
		c.st.cwd = resolveDir(c.st.home, c.st.cwd, c.args[0].Value)
	}
	c.st.prev = prev
}

// interpreterC blocks sh -c and its kin: the command string never surfaces
// as its own Bash tool call, bypassing the allow list. Short-option
// clusters only.
func (c *command) interpreterC() error {
	for _, v := range c.values() {
		if v == "--" || !strings.HasPrefix(v, "-") {
			break
		}
		if !strings.HasPrefix(v, "--") && strings.Contains(v, "c") {
			return c.block("interpreter -c wrapping bypasses the permission allow list; run the command directly as a Bash tool call", "interpreter-c-wrap")
		}
	}
	return nil
}

// inPlaceEdit guards sed -i and sd, which is always in place when given a
// file: blocked on a shell rc file, asked on the overlay.
func (c *command) inPlaceEdit() error {
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
// gitSubcommand splits git's words past its global options: the
// subcommand, its arguments, and the -C directory.
func gitSubcommand(words []string) (sub string, args []string, dir string) {
	for i := 0; i < len(words); {
		w := words[i]
		switch {
		case w == "-C":
			if i+1 < len(words) {
				dir = words[i+1]
			}
			i += 2
		case slices.Contains([]string{"-c", "--git-dir", "--work-tree", "--namespace", "--config-env", "--super-prefix"}, w):
			i += 2
		case strings.HasPrefix(w, "-"):
			i++
		default:
			return w, words[i+1:], dir
		}
	}
	return "", nil, dir
}

func (c *command) git() error {
	sub, args, dir := gitSubcommand(c.values())
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
			p.forced = append(p.forced, "git push --force. Use --force-with-lease if you must.")
		case a == "--mirror":
			p.forced = append(p.forced, "git push --mirror overwrites every remote ref, protected branches included")
		case a == "--tags":
			p.tagsOnly = true
		case slices.Contains([]string{"--repo", "--push-option", "--receive-pack", "--exec"}, a):
			wantValue = true
		case strings.HasPrefix(a, "--"):
		default:
			if strings.Contains(a, "f") {
				p.forced = append(p.forced, "git push -f. Use --force-with-lease if you must.")
			}
			wantValue = strings.HasSuffix(a, "o")
		}
	}
	for _, ref := range p.refspecs {
		if strings.HasPrefix(ref, "+") {
			p.forced = append(p.forced, "force push via a +refspec. Use --force-with-lease if you must.")
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
			if err := c.block("push to a protected branch; use a feature branch", "git-push-protected"); err != nil {
				return err
			}
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
