package bashguard

import (
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
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
	rest   string   // normalized source after the name, to the pipeline's end
	text   string   // name + rest: what the per-command regexes read
	alone  bool     // the only command of its pipeline, outside a subshell
	// assigns are the VAR=value words the command runs with: its own, and
	// an env wrapper's before its name.
	assigns []string
}

func (c *command) block(reason, rule string) error { return c.st.h.Block(reason, rule) }

// shortCluster is true when a is a short-option cluster (-xyz) holding any
// of letters.
func shortCluster(a, letters string) bool {
	return strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsAny(a, letters)
}

// readsSensitive blocks when any of paths, resolved against the cwd, is a
// credential file. nil when none is, or when sensitive-read is disabled, so
// the caller carries on.
func (c *command) readsSensitive(paths []string) error {
	for _, p := range paths {
		if guard.IsSensitive(c.st.cfg, c.st.home, c.st.abs(unquote(p))) {
			return c.block("reading a sensitive file is not permitted", "sensitive-read")
		}
	}
	return nil
}

// globsSensitive blocks a read of an unquoted glob that names a credential
// file by name (cat .e*), whatever the directory holds: the shell expands
// it into paths the guard never sees.
func (c *command) globsSensitive(ops []string) error {
	for _, a := range c.args {
		if !a.literal && strings.ContainsAny(a.Value, "*?[{") && slices.Contains(ops, a.Value) &&
			guard.SearchesSensitive(c.st.cfg, c.st.home, "", a.Value, false) {
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

// readers only read the files they're given: the pure readers, and the
// pagers and editors that write only on a flag. Any other command naming a
// kit switch gets a prompt, since a write there can turn the kit off.
var readers = func() map[string]bool {
	m := maps.Clone(classify.PureReaders)
	for _, name := range []string{"bat", "less", "more", "file", "sed", "sd"} {
		m[name] = true
	}
	return m
}()

// metadataOnly commands act on a file's name or metadata, or use a key
// without printing it, so naming a credential file is no read. git is
// read per subcommand, by gitReads.
var metadataOnly = map[string]bool{
	"ls": true, "stat": true, "test": true, "[": true, "touch": true, "rm": true, "rmdir": true,
	"chmod": true, "chown": true, "mkdir": true, "realpath": true, "readlink": true, "file": true,
	"wc": true, "du": true, "basename": true, "dirname": true, "echo": true, "printf": true, "cd": true,
	"tee": true, "ssh": true, "ssh-add": true,
}

// patternFirst is how many leading operands are a pattern, script, or
// filter rather than a path.
var patternFirst = map[string]int{"grep": 1, "rg": 1, "sed": 1, "awk": 1, "jq": 1, "yq": 1, "sd": 2, "git grep": 1}

// patternFlag is how a patternFirst command takes its pattern or script from
// a flag instead, which leaves every operand a path: the short letters giving
// it inline (pattern) or from a file (file), the short letters whose value
// ends their cluster (valued), and the long names, true for a file.
type patternFlag struct {
	pattern, file, valued string
	long                  map[string]bool
}

var patternFlags = map[string]patternFlag{
	"grep":     {"e", "f", "ABCDdm", map[string]bool{"--regexp": false, "--file": true}},
	"git grep": {"e", "f", "ABCm", map[string]bool{"--file": true}},
	"rg":       {"e", "f", "ABCEMTgjmrt", map[string]bool{"--regexp": false, "--file": true}},
	"sed":      {"e", "f", "il", map[string]bool{"--expression": false, "--file": true}},
	"awk":      {"e", "f", "vF", map[string]bool{"--source": false, "--file": true}},
}

// withoutPatternFlags is values with name's pattern flags and their inline
// patterns taken out, a script file kept as an operand since it's read, and
// whether any flag gave the pattern.
func withoutPatternFlags(name string, values []string) ([]string, bool) {
	spec, ok := patternFlags[name]
	if !ok {
		return values, false
	}
	var out []string
	given := false
	for i := 0; i < len(values); i++ {
		v := values[i]
		if v == "--" {
			return append(out, values[i:]...), given
		}
		long, value, attached := strings.Cut(v, "=")
		if isFile, ok := spec.long[long]; ok {
			given = true
			if !attached && i+1 < len(values) {
				i++
				value = values[i]
			}
			if isFile {
				out = append(out, value)
			}
			continue
		}
		value, isFile, found := spec.inCluster(v)
		if !found {
			out = append(out, v)
			continue
		}
		given = true
		if value == "" && i+1 < len(values) {
			i++
			value = values[i]
		}
		if isFile {
			out = append(out, value)
		}
	}
	return out, given
}

// inCluster finds a pattern or file letter in the short-option cluster v:
// the inline rest of v after it, "" when its value is the next word.
func (spec patternFlag) inCluster(v string) (value string, isFile, found bool) {
	if !strings.HasPrefix(v, "-") || strings.HasPrefix(v, "--") {
		return "", false, false
	}
	for j := 1; j < len(v); j++ {
		letter := v[j]
		isFile = strings.IndexByte(spec.file, letter) >= 0
		if isFile || strings.IndexByte(spec.pattern, letter) >= 0 {
			return v[j+1:], isFile, true
		}
		if strings.IndexByte(spec.valued, letter) >= 0 {
			break
		}
	}
	return "", false, false
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
	if err := c.readsSensitive(c.readOperands()); err != nil {
		return err
	}
	if err := c.globsSensitive(c.readOperands()); err != nil {
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
	if err := c.globalInstall(); err != nil {
		return err
	}
	if weaken.UpdatesSnapshots(c.st.cfg.WeakenedChecks, strings.Join(append([]string{c.name}, c.values()...), " ")) {
		c.st.ask("this rewrites every snapshot the run disagrees with, so the tests would accept whatever the code prints now; confirm the new output is what you want")
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

// commandChecks are the checks for one command name; any other name gets
// the shell, mkfs.*, and package-manager checks.
var commandChecks = map[string]func(*command) error{
	"cd":     func(c *command) error { c.cd(); return nil },
	"rm":     (*command).rm,
	"dd":     (*command).diskTool,
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

// kit asks before git-base flags that can write or run programs, and blocks
// run-checks, which the hooks run.
func (c *command) kit() error {
	words := strings.Fields(c.rest)
	switch {
	case len(words) == 0:
	case words[0] == "git-base" && !gitBaseFlagsSafe(words[1:]):
		// An explicit ask: with no decision, a Bash(kit git-base *) allow
		// rule would approve --output=<file> silently.
		c.st.ask("kit git-base passes this flag to git, which can write files or run programs; confirm it")
	case words[0] == "run-checks" && !slices.Contains(words[1:], "--plan"):
		return c.block("the hooks run the gates and show the user every result\n"+layout.KV("do", "don't run them yourself; kit run-checks --plan only lists them"), "run-checks-by-hook")
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

func (c *command) readOperands() []string { return readOperands(c.name, c.values()) }

// readOperands are the operands command name reads as files when run with
// values: none for a metadataOnly command or a git subcommand that prints
// none, and never a leading pattern unless a flag gave it. git show
// HEAD:.env names .env, and so does kit read git show HEAD:.env.
func readOperands(name string, values []string) []string {
	if name == "kit" && len(values) > 1 && values[0] == "read" && values[1] == "git" {
		name, values = "git", values[2:]
	}
	git := name == "git"
	if git {
		sub, args, _ := classify.GitSubcommand(values)
		if !gitReads[sub] {
			return nil
		}
		name, values = "git "+sub, args
	}
	values, given := withoutPatternFlags(name, values)
	ops := nonOptions(values)
	if git {
		for i, op := range ops {
			if _, path, ok := strings.Cut(op, ":"); ok {
				ops[i] = path
			}
		}
	}
	if metadataOnly[name] {
		return nil
	}
	if name == "curl" {
		return append(ops, curlFiles(values)...)
	}
	// ln and mv read their sources: the key reached by a new name is read
	// under it later. Their target is only written.
	if name == "ln" || name == "mv" {
		return sources(values)
	}
	if given {
		return ops
	}
	return ops[min(patternFirst[name], len(ops)):]
}

// curlFiles are the files curl sends: what follows the first @ in a word
// (-d @f, -d@f, --data-binary=@f, -F name=@f;type=x). A URL's user@host
// yields its host, which names no credential file.
func curlFiles(values []string) []string {
	var files []string
	for _, v := range values {
		if _, file, ok := strings.Cut(v, "@"); ok && file != "" {
			file, _, _ = strings.Cut(file, ";")
			files = append(files, file)
		}
	}
	return files
}

// sources are the operands ln or mv reads: all but the target, which is
// the last one unless -t or --target-directory names it. -S and --suffix
// take a value too.
func sources(values []string) []string {
	var ops []string
	targeted := false
	for i := 0; i < len(values); i++ {
		v := values[i]
		switch {
		case v == "--":
			ops = append(ops, values[i+1:]...)
			i = len(values)
		case strings.HasPrefix(v, "--"):
			// GNU takes any unambiguous prefix: --target, --suf.
			name, _, attached := strings.Cut(v, "=")
			isTarget := len(name) > 2 && strings.HasPrefix("--target-directory", name)
			targeted = targeted || isTarget
			if !attached && (isTarget || len(name) > 2 && strings.HasPrefix("--suffix", name)) {
				i++
			}
		case strings.HasPrefix(v, "-"):
			// In a cluster, the first value-taking letter takes the rest of
			// the word, or the next word when it's last.
			if at := strings.IndexAny(v, "tS"); at > 0 {
				targeted = targeted || v[at] == 't'
				if at == len(v)-1 {
					i++
				}
			}
		default:
			ops = append(ops, v)
		}
	}
	if !targeted && len(ops) > 1 {
		ops = ops[:len(ops)-1]
	}
	return ops
}

// cd tracks the directory a later command runs in, so it checks the right
// repo; a cd inside a pipeline or a subshell moves nothing after it.
func (c *command) cd() {
	if c.alone {
		c.st.cd(c.args)
	}
}

// interpreterC blocks sh -c and its kin: the command string never surfaces
// as its own Bash tool call, bypassing the allow list. A -c in any
// short-option cluster before the script counts, past the values that -o,
// +O, --rcfile, and their kin take.
func (c *command) interpreterC() error {
	if wrap, _ := shellMode(c.values()); wrap {
		return c.block(wrapReason, "interpreter-c-wrap")
	}
	return nil
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
		if guard.IsRCFile(c.st.cfg, c.st.home, p) {
			if err := c.block("in-place edit of a shell rc file\n"+layout.KV("do", "edit it in the dotfiles repo"), "rc-inplace-edit"); err != nil {
				return err
			}
		}
		c.st.askKitSwitch(p)
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
		if guard.IsRCFile(c.st.cfg, c.st.home, p) {
			return c.block(rcWriteReason, "rc-redirect")
		}
	}
	return nil
}

// writeFlags holds the commands that write only with one of these
// arguments; without one, they read whatever paths they name.
var writeFlags = map[string]func(string) bool{
	"yq": func(v string) bool {
		flag, _, _ := strings.Cut(v, "=")
		return shortCluster(flag, "is") || strings.HasPrefix(v, "--inplace") || strings.HasPrefix(v, "--split-exp")
	},
	"find": func(v string) bool {
		return v == "-delete" || v == "-fls" || strings.HasPrefix(v, "-exec") || strings.HasPrefix(v, "-ok") || strings.HasPrefix(v, "-fprint")
	},
	// sort takes any unambiguous prefix of a long option (--out=x).
	"sort": func(v string) bool {
		return shortCluster(v, "o") || abbrevOf(v, "--output") || abbrevOf(v, "--compress-program")
	},
	"sg":       sgWriteFlag,
	"ast-grep": sgWriteFlag,
	"fd":       func(v string) bool { return shortCluster(v, "xX") || strings.HasPrefix(v, "--exec") },
	"tree":     func(v string) bool { return shortCluster(v, "oR") },
}

// abbrevOf is true when v, up to any "=", is long or a getopt_long prefix of it.
func abbrevOf(v, long string) bool {
	flag, _, _ := strings.Cut(v, "=")
	return len(flag) > 2 && strings.HasPrefix(long, flag)
}

func sgWriteFlag(v string) bool {
	return shortCluster(v, "Ui") || v == "--update-all" || v == "--interactive"
}

// writes is false only for a writeFlags command run without its write
// arguments; sg new scaffolds files, so it writes too.
func (c *command) writes() bool {
	flag, ok := writeFlags[c.name]
	if !ok {
		return true
	}
	if ops := c.operands(); (c.name == "sg" || c.name == "ast-grep") && len(ops) > 0 && ops[0] == "new" {
		return true
	}
	return slices.ContainsFunc(c.values(), flag)
}

// fileWriters are the commands whose operands include the files they write.
var fileWriters = map[string]bool{"cp": true, "mv": true, "tee": true, "install": true, "ln": true, "touch": true,
	"mkdir": true, "rm": true, "rsync": true, "truncate": true, "dd": true, "patch": true,
	"chmod": true, "chown": true, "unlink": true}

// overlayWrite asks when a known writer names a kit switch
// (guard.KitSwitch): a fileWriters command, perl -i, a writeFlags command
// with its flags, or git through checkout/restore/apply/mv/rm/stash/reset.
// Any other command naming one goes through, logged as a kit-switch shadow:
// most only read, and guessing their operands asked on every log query.
func (c *command) overlayWrite() {
	all := append([]string{c.name}, c.values()...)
	switch {
	case !c.writes():
		return
	case c.name == "git":
		if !gitWriters.MatchString(" " + strings.Join(all, " ") + " ") {
			return
		}
	case readers[c.name]:
		return
	// A cd the guard follows writes nothing, and later paths resolve
	// against where it went; an untracked one would resolve them wrongly.
	case c.name == "cd" && c.alone && c.st.tracks(c.args):
		return
	case c.name == "cd" || c.name == "pushd":
		for _, v := range c.values() {
			if what := c.st.kitSwitch(v); what != "" {
				c.st.ask("this moves into " + what + " where the guard can't follow later paths, so a write there would go unchecked; confirm the change")
				return
			}
		}
		return
	}
	_, flagged := writeFlags[c.name]
	known := fileWriters[c.name] || flagged || c.name == "git" || c.name == "perl" && perlInPlace(c.values())
	for _, v := range c.values() {
		_, after, hasEq := strings.Cut(v, "=")
		if known {
			if c.st.askKitSwitch(v) || hasEq && c.st.askKitSwitch(after) {
				return
			}
			continue
		}
		if c.st.kitSwitch(v) != "" || hasEq && c.st.kitSwitch(after) != "" {
			if !c.st.shadowed {
				c.st.shadowed = true
				c.st.h.Shadow("kit-switch")
			}
			return
		}
	}
}

// perlInPlace is true when one of perl's option clusters holds -i.
func perlInPlace(values []string) bool {
	for _, v := range values {
		if v == "--" {
			return false
		}
		if strings.HasPrefix(v, "-") && !strings.HasPrefix(v, "--") && strings.Contains(v, "i") {
			return true
		}
	}
	return false
}

func (c *command) rm() error {
	// Whole words only: rm -rf *.log and rm -rf dist/* are asked, not blocked.
	force, recursive, broad := false, false, false
	for _, v := range c.values() {
		switch {
		case v == "--recursive":
			force, recursive = true, true
		case v == "--force":
			force = true
		case strings.HasPrefix(v, "--"):
		case strings.HasPrefix(v, "-"):
			force = force || strings.ContainsAny(v, "rRfF")
			recursive = recursive || strings.ContainsAny(v, "rR")
		case slices.Contains([]string{"/", "/*", "~", "~/", "~/*", "$HOME", "${HOME}", "$HOME/", "${HOME}/", "$HOME/*", "${HOME}/*", ".", "..", "./", "../", "*"}, v):
			broad = true
		}
	}
	if force && broad {
		return c.block("rm with recursive force against root, home, or cwd", "rm-recursive")
	}
	if recursive && !c.st.allScratch(nonOptionsOf(c.args, func(w Word) string { return w.Value })) {
		c.st.ask("rm -r deletes a directory tree; confirm the path")
	}
	return nil
}

// plainWord is a word with no shell syntax in it.
var plainWord = regexp.MustCompile(`^[A-Za-z0-9_./%:=+@,-]+$`)

// mktempDir is true for a whole $(mktemp ...) call with -d or --directory
// and only plain words, so nothing else runs in it.
func mktempDir(value string) bool {
	if !strings.HasPrefix(value, "$(") || !strings.HasSuffix(value, ")") {
		return false
	}
	inner := value[2 : len(value)-1]
	words := strings.Fields(inner)
	if len(words) < 2 || words[0] != "mktemp" || slices.ContainsFunc(words, func(w string) bool { return !plainWord.MatchString(w) }) {
		return false
	}
	return slices.ContainsFunc(words[1:], func(w string) bool {
		return w == "--directory" || strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "--") && strings.Contains(w, "d")
	})
}

// allScratch is true when ops is non-empty and each one is a scratch
// operand.
func (st *state) allScratch(ops []Word) bool {
	return len(ops) > 0 && !slices.ContainsFunc(ops, func(w Word) bool { return !st.scratchOperand(w) })
}

// quotedVar is a whole word that is one variable in double quotes, "$S" or
// "${S}": unquoted, IFS could split it into other paths.
var quotedVar = regexp.MustCompile(`^"\$(?:([A-Za-z_][A-Za-z0-9_]*)|\{([A-Za-z_][A-Za-z0-9_]*)\})"$`)

// sourcing is a call that can set any variable unseen: eval, source, or ".".
var sourcing = regexp.MustCompile(`(^|[;&|({\s])(eval|source|\.)\s`)

// scratchOperand is true for a scratch target, quoted when it expands
// anything, or a quoted variable set once, outside any loop, to a scratch
// target or a fresh mktemp -d dir, and named nowhere else in the command
// (read S, printf -v S, declare S=/ all set it too).
func (st *state) scratchOperand(w Word) bool {
	quoted := strings.HasPrefix(w.Raw, `"`) && strings.HasSuffix(w.Raw, `"`)
	m := quotedVar.FindStringSubmatch(w.Raw)
	if m == nil {
		return !varRef.MatchString(w.Value) && (quoted || !strings.Contains(w.Value, "$")) && st.scratchTarget(w.Value)
	}
	name := m[1] + m[2]
	for _, loop := range st.loopVars {
		if _, ok := loop[name]; ok {
			return false
		}
	}
	value, ok := st.vars[name]
	return ok && (mktempDir(value) || st.scratchTarget(value)) && st.setOnly(name)
}

// setOnly is true when the command names name only in one assignment and
// in $name references, and runs nothing that could set it unseen.
func (st *state) setOnly(name string) bool {
	cmd := st.h.Payload.String(hook.KeyInputCommand)
	if sourcing.MatchString(cmd) {
		return false
	}
	q := regexp.QuoteMeta(name)
	count := func(pattern string) int { return len(regexp.MustCompile(pattern).FindAllStringIndex(cmd, -1)) }
	sets := count(`\b` + q + `=`)
	return sets == 1 && count(`\b`+q+`\b`) == sets+count(`\$`+q+`\b|\$\{`+q+`\}`)
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
