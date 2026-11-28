package bashguard

import (
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/weaken"
)

// command is one command of a pipeline, past its assignments and wrappers.
type command struct {
	st     *state
	name   string // basename, quotes removed: "rm", r''m, /bin/rm all read rm
	args   []Word
	redirs []Redir
	inputs []string // < redirect sources
	rest   string   // normalized source after the name, to the command's end
	text   string   // name + rest: what the per-command regexes read
	alone  bool     // the only command of its pipeline, outside a subshell
	// assigns are the VAR=value words the command runs with: its own, and
	// an env wrapper's before its name.
	assigns []string
	// shellString is true when the parser read the shell's -c string as
	// commands (Call.shellString), so its rules already ran on them.
	shellString bool
	// movesShell is true when a cd here can move the shell running what
	// follows (the movesShell func).
	movesShell bool
}

func (c *command) block(reason, rule string) error { return c.st.h.Block(reason, rule) }

// shortCluster is true when a is a short-option cluster (-xyz) holding any
// of letters.
func shortCluster(a, letters string) bool {
	return strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsAny(a, letters)
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
func (c *command) operands() []string { return nonOptions(c.values()) }

var (
	chmod777    = regexp.MustCompile(`chmod[[:space:]]+(-R[[:space:]]+)?777([[:space:]]|$)`)
	chmodSpace  = regexp.MustCompile(`chmod[[:space:]]`)
	chmodBroad  = regexp.MustCompile(`[[:space:]]["']?(\.|\.\.|/)["']?($|[[:space:]])`)
	chmodHome   = regexp.MustCompile(`[[:space:]]["']?(~|\$HOME|\$\{HOME\})["']?($|[[:space:]]|/)`)
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
	gitWriters  = regexp.MustCompile(` (checkout|restore|apply|mv|rm|stash|reset) `)
	forkBomb    = regexp.MustCompile(`:\(\)[[:space:]]*\{`)
	deviceWrite = regexp.MustCompile(`>[[:space:]]*/dev/(sd|nvme|disk|rdisk)`)
	xargsRm     = regexp.MustCompile(`xargs[[:space:]]+((-[^[:space:]]+[[:space:]]+)*)rm[[:space:]]+-[a-zA-Z]*[rRfF]`)
	sqQuoted    = regexp.MustCompile(`'[^']*'`)
	dqQuoted    = regexp.MustCompile(`"[^"]*"`)
)

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
	for _, check := range []func() error{c.reads, c.rcWrite, c.destructive, c.globalInstall} {
		if err := check(); err != nil {
			return err
		}
	}
	// A word holding a space is one argument, a search pattern or a
	// message, never the runner and its flags.
	words := slices.DeleteFunc(append([]string{c.name}, c.values()...), func(w string) bool { return strings.ContainsAny(w, " \t\n") })
	if weaken.UpdatesSnapshots(c.st.cfg.WeakenedChecks, strings.Join(words, " ")) {
		c.st.ask("this rewrites every snapshot the run disagrees with, so the tests would accept whatever the code prints now; confirm the new output is what you want", "snapshot-rewrite")
	}
	if check, ok := commandChecks[c.name]; ok {
		return check(c)
	}
	if runsC(c.name) {
		return c.interpreterC()
	}
	if strings.HasPrefix(c.name, "mkfs.") {
		return c.diskTool()
	}
	return c.packageManager()
}

// reads blocks a credential the command reads. Any command prints what <
// feeds it: sort < .env reads it as well as cat.
func (c *command) reads() error {
	if err := c.readsSensitive(c.inputs); err != nil {
		return err
	}
	if err := c.readsSensitive(c.readOperands()); err != nil {
		return err
	}
	return c.globsSensitive(c.readOperands())
}

// destructive blocks each destructive pattern the command matches; a find
// -delete that walks only scratch passes.
func (c *command) destructive() error {
	for _, d := range destructive {
		if d.rule == "find-delete" && c.findsOnlyScratch() {
			continue
		}
		if slices.Contains(d.names, c.name) && !slices.ContainsFunc(d.all, func(re *regexp.Regexp) bool { return !re.MatchString(c.text) }) {
			if err := c.block(strings.ReplaceAll(d.reason, "%s", c.name), d.rule); err != nil {
				return err
			}
		}
	}
	return nil
}

// commandChecks are the checks for one command name; any other name gets
// the shell, mkfs.*, and package-manager checks.
var commandChecks = map[string]func(*command) error{
	"cd":    func(c *command) error { c.cd(); return nil },
	"pushd": func(c *command) error { c.cd(); return nil },
	"popd":  func(c *command) error { c.cd(); return nil },
	"rm":    (*command).rm,
	"rsync": (*command).rsync,
	"dd": func(c *command) error {
		// dd with no of= writes only to stdout, as in dd if=bin skip=N | strings,
		// so it's a reader of its if= file.
		var ins []string
		for _, v := range c.values() {
			if in, ok := strings.CutPrefix(v, "if="); ok {
				ins = append(ins, in)
			}
		}
		if err := c.readsSensitive(ins); err != nil {
			return err
		}
		if slices.ContainsFunc(c.values(), func(v string) bool { return strings.HasPrefix(v, "of=") }) {
			return c.diskTool()
		}
		return nil
	},
	"shred":  (*command).diskTool,
	"wipefs": (*command).diskTool,
	"mkfs":   (*command).diskTool,
	"chmod":  (*command).chmod,
	"git":    (*command).git,
	"curl":   (*command).curl,
	"wget":   (*command).wget,
	"eval": func(c *command) error {
		return c.block("eval runs a command string that bypasses the permission allow list\n"+layout.KV("do", "run the command directly as a Bash tool call"), "interpreter-c-wrap")
	},
	"su": func(c *command) error {
		// Anywhere: after the user, and after --, the words go to the shell.
		if slices.ContainsFunc(c.values(), commandOption) {
			return c.block(wrapReason, "interpreter-c-wrap")
		}
		return nil
	},
	"kit": (*command).kit,
	"sed": (*command).inPlaceEdit,
	"sd":  (*command).inPlaceEdit,
	"tee": (*command).tee,
}

func (c *command) diskTool() error { return c.block("low level disk or filesystem tool", "disk-tool") }

// kit asks before git-base flags that can write or run programs.
func (c *command) kit() error {
	words := strings.Fields(c.rest)
	switch {
	case len(words) == 0:
	case words[0] == "git-base" && !gitBaseFlagsSafe(words[1:]):
		// An explicit ask: with no decision, a Bash(kit git-base *) allow
		// rule would approve --output=<file> silently.
		c.st.ask("kit git-base passes this flag to git, which can write files or run programs; confirm it", "kit-git-base")
	}
	return nil
}

// tee asks before each file it writes loose in the current directory.
func (c *command) tee() error {
	for _, p := range c.operands() {
		c.st.looseWrite("tee", p)
	}
	return nil
}

// cd tracks the directory a later command runs in, after a cd, pushd, or
// popd, so it checks the right repo. One before a pipeline's last command
// moves nothing; one ending a pipeline (zsh runs it in the current shell)
// or inside a subshell loses the cwd, until the subshell ends.
func (c *command) cd() {
	switch {
	case !c.movesShell:
	case c.alone:
		c.st.moveAt(c.st.segs, c.st.at, c.name, c.st.pinWords(c.args))
	default:
		c.st.loseIn(c.st.segs[c.st.at])
	}
}

// interpreterC blocks sh -c and its kin: the command string never surfaces
// as its own Bash tool call, bypassing the allow list. A -c in any
// short-option cluster before the script counts, past the values that -o,
// +O, --rcfile, and their kin take. A POSIX shell's literal string passes:
// the parser read it as commands, so every rule checked it (shellCString).
func (c *command) interpreterC() error {
	if wrap, _ := shellMode(c.values()); wrap && !c.shellString {
		return c.block(wrapReason, "interpreter-c-wrap")
	}
	return nil
}

// cString is the index in values of the string a shell's -c runs, or -1:
// no -c, nothing after it, or a -c clustered with -o or -O (-oc), where
// which word is the string isn't clear.
func cString(values []string) int {
	wrap := false
	for i := 0; i < len(values); i++ {
		v := values[i]
		switch {
		case v == "--":
			return indexIf(wrap && i+1 < len(values), i+1)
		case v == "-":
			return -1
		case !strings.HasPrefix(v, "-") && !strings.HasPrefix(v, "+"):
			return indexIf(wrap, i)
		case v == "--rcfile" || v == "--init-file":
			i++ // its file, not an option
		case strings.HasPrefix(v, "--"):
		case v[0] == '-' && strings.Contains(v, "c"):
			if strings.ContainsAny(v, "oO") {
				return -1
			}
			wrap = true
		case strings.ContainsAny(v[len(v)-1:], "oO"):
			i++ // -o pipefail, +O extglob: the option's name
		}
	}
	return -1
}

// indexIf is i when ok, else -1.
func indexIf(ok bool, i int) int {
	if ok {
		return i
	}
	return -1
}

var wrapReason = "interpreter -c wrapping bypasses the permission allow list\n" + layout.KV("do", "run the command directly as a Bash tool call")

// rcWriteReason blocks a write to a shell rc file, by redirect or command.
var rcWriteReason = "direct write to a shell rc file\n" + layout.KV("do", "use the dotfiles repo")

// shellMode reads a shell's arguments: wrap when a -c runs a string, script
// when an operand names a script file; neither, it reads stdin.
func shellMode(values []string) (wrap, script bool) {
	for i := 0; i < len(values); i++ {
		v := values[i]
		switch {
		case v == "--":
			return false, i+1 < len(values)
		case v == "-":
			return false, false
		case !strings.HasPrefix(v, "-") && !strings.HasPrefix(v, "+"):
			return false, true
		case v == "--rcfile" || v == "--init-file":
			i++ // its file, not an option
		case strings.HasPrefix(v, "--"):
		case v[0] == '-' && strings.Contains(v, "c"):
			return true, false
		case v[0] == '-' && strings.Contains(v, "s"):
			return false, false // -s: operands are positional parameters
		case strings.ContainsAny(v[len(v)-1:], "oO"):
			i++ // -o pipefail, +O extglob: the option's name
		}
	}
	return false, false
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

func nonOptions(words []string) []string {
	return nonOptionsOf(words, func(w string) string { return w })
}

// nonOptionsOf is nonOptions over items, each read through value.
func nonOptionsOf[T any](items []T, value func(T) string) []T {
	var out []T
	done := false
	for _, item := range items {
		w := value(item)
		if !done && w == "--" {
			done = true
			continue
		}
		if !done && strings.HasPrefix(w, "-") {
			continue
		}
		out = append(out, item)
	}
	return out
}
