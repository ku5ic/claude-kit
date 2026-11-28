package bashguard

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/guard"
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

// readsSensitive blocks when any of paths, resolved against the cwd, is a
// credential file outside scratch. nil when none is, or when sensitive-read
// is disabled, so the caller carries on.
func (c *command) readsSensitive(paths []string) error {
	for _, p := range paths {
		if guard.SensitiveOutsideScratch(c.st.cfg, c.st.h.Paths, c.st.home, c.st.abs(unquote(p))) {
			return c.block("reading a sensitive file is not permitted", "sensitive-read")
		}
	}
	return nil
}

// zshOperator is true when a word holds a zsh extendedglob operator, which a .zshrc can turn on and bash parses as text: # or ^
// anywhere, or ~ with a name on both sides (*~x). A trailing ~, an editor
// backup's name, isn't one.
func zshOperator(word string) bool {
	if strings.ContainsAny(word, "#^") {
		return true
	}
	return len(word) > 2 && strings.Contains(word[1:len(word)-1], "~")
}

// globsSensitive blocks a read of an unquoted glob the shell expands into a
// credential file. Where the guard knows the cwd and can expand the glob
// itself, the files it matches now decide (cat docs/adr/*12* reads no
// *.p12); otherwise a glob that names one blocks (cat .e*).
func (c *command) globsSensitive(ops []string) error {
	for _, a := range c.args {
		// The parser keeps no record of which bytes were quoted, so a quoted
		// operator counts too (g/".e"#nv quotes only part of the word).
		glob := !a.literal && strings.ContainsAny(a.Value, "*?[{") || zshOperator(a.Value)
		if !glob || !slices.Contains(ops, a.Value) {
			continue
		}
		hits, ok := c.st.expandGlob(a.Value)
		if ok && !slices.ContainsFunc(hits, func(p string) bool {
			return guard.SensitiveOutsideScratch(c.st.cfg, c.st.h.Paths, c.st.home, p)
		}) {
			continue
		}
		if ok || guard.SearchesSensitive(c.st.cfg, c.st.home, "", a.Value, false) {
			return c.block("reading a sensitive file is not permitted", "sensitive-read")
		}
	}
	return nil
}

// expandGlob is every file glob could match from the cwd, whatever the
// shell's options: names compare in any case and dotfiles count, so
// nocaseglob, CASE_GLOB, and dotglob can only shrink what the shell
// expands. A name holding a zsh extendedglob operator (# ^ ~, which a
// .zshrc can turn on) counts as *, any file in its directory, since the
// guard doesn't read them. ok is false when the guard can't do that: the
// cwd is lost or isn't a directory it can read (cd "$X"), the glob starts
// at ~ or holds an expansion, a zsh ** or qualifier, too many {a,b}, a
// bracket, whose classes differ between shells ([!x], [[:alpha:]]), or a
// wildcard or extendedglob operator before its last "/".
func (st *state) expandGlob(glob string) (hits []string, ok bool) {
	if st.lost || !fsx.IsDir(st.cwd) || strings.HasPrefix(glob, "~") || strings.ContainsAny(glob, "$`()[") || strings.Contains(glob, "**") {
		return nil, false
	}
	alts, ok := guard.Braces(glob)
	if !ok {
		return nil, false
	}
	for _, alt := range alts {
		dir, base := filepath.Split(st.abs(alt))
		if strings.ContainsAny(dir, "*?#^~") {
			return nil, false
		}
		if zshOperator(base) {
			base = "*"
		}
		entries, err := os.ReadDir(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue // the shell passes the pattern as written, a name no file has
		}
		if err != nil {
			return nil, false
		}
		for _, e := range entries {
			if guard.Glob(strings.ToLower(base), strings.ToLower(e.Name())) {
				hits = append(hits, filepath.Join(dir, e.Name()))
			}
		}
	}
	return hits, true
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
		if d.rule == "find-delete" && c.findsOnlyScratch() {
			continue
		}
		if slices.Contains(d.names, c.name) && !slices.ContainsFunc(d.all, func(re *regexp.Regexp) bool { return !re.MatchString(c.text) }) {
			if err := c.block(strings.ReplaceAll(d.reason, "%s", c.name), d.rule); err != nil {
				return err
			}
		}
	}
	if err := c.globalInstall(); err != nil {
		return err
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
			if wrap && i+1 < len(values) {
				return i + 1
			}
			return -1
		case v == "-":
			return -1
		case !strings.HasPrefix(v, "-") && !strings.HasPrefix(v, "+"):
			if wrap {
				return i
			}
			return -1
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
var fileWriters = nameSet("cp", "mv", "tee", "install", "ln", "touch", "mkdir", "rm", "rsync", "truncate", "dd", "patch",
	"chmod", "chown", "unlink", "sponge", "ed", "ex", "vi", "vim", "nvim")

// nameSet is names as a set.
func nameSet(names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

// overlayWrite asks when a known writer names a kit switch
// (guard.KitSwitch): a fileWriters command, an inPlaceScript, a writeFlags command
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
				c.st.ask("this moves into "+what+" where the guard can't follow later paths, so a write there would go unchecked; confirm the change", "kit-switch")
				return
			}
		}
		return
	}
	_, flagged := writeFlags[c.name]
	known := fileWriters[c.name] || flagged || c.name == "git" || c.inPlaceScript()
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

// inPlaceScript is true for a script runner editing its file operands in
// place: perl or ruby -i, or awk loading gawk's inplace extension.
func (c *command) inPlaceScript() bool {
	values := c.values()
	switch c.name {
	case "perl", "ruby":
		return inPlace(values)
	case "awk", "gawk":
		for i, v := range values {
			if v == "-iinplace" || v == "--include=inplace" || (v == "-i" || v == "--include") && i+1 < len(values) && values[i+1] == "inplace" {
				return true
			}
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
	if recursive && (c.st.linksBefore(c.st.at) || !c.st.allScratch(nonOptionsOf(c.args, func(w Word) string { return w.Value }))) {
		c.st.ask("rm -r deletes a directory tree; confirm the path", "rm-recursive")
	}
	return nil
}

// rsync asks when it deletes outside scratch: --del and --delete* in its
// destination, --remove-source-files in its sources. An option after an
// operand asks too, since its value could pass for the destination.
func (c *command) rsync() error {
	deletes, removesSources, late := false, false, false
	var ops []Word
	ended := false
	for _, w := range c.args {
		switch v := w.Value; {
		case !ended && v == "--":
			ended = true
		case !ended && strings.HasPrefix(v, "-"):
			late = late || len(ops) > 0
			deletes = deletes || v == "--del" || strings.HasPrefix(v, "--delete")
			removesSources = removesSources || v == "--remove-source-files" || v == "--remove-sent-files"
		default:
			ops = append(ops, w)
		}
	}
	if !deletes && !removesSources {
		return nil
	}
	if late || len(ops) < 2 ||
		deletes && !c.st.allScratch(ops[len(ops)-1:]) ||
		removesSources && !c.st.allScratch(ops[:len(ops)-1]) {
		c.st.ask("rsync deletes files outside scratch; confirm the path", "rsync-delete")
	}
	return nil
}

// linksBefore is true when an ln in the command can run before or beside
// segment at. A link it makes isn't on disk yet for scratchTarget to
// follow. A segment in a loop, a function body, a branch that may not run,
// or a subshell (one backgrounded with & included) can follow any ln.
func (st *state) linksBefore(at int) bool {
	anywhere := st.segs[at].loop != 0 || st.segs[at].conditional || st.segs[at].subshell
	for i, seg := range st.segs {
		if i > at && !anywhere {
			break
		}
		if slices.ContainsFunc(seg.Calls, func(call Call) bool {
			j := lead(call.Words)
			return j < len(call.Words) && baseName(call.Words[j].Value) == "ln"
		}) {
			return true
		}
	}
	return false
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

// quotedVar is a whole word that is one variable in double quotes, "$S",
// "${S}", or "${S:?}", and maybe a plain path after it, "$S/x": unquoted,
// IFS could split it into other paths. Its groups are the name (twice),
// the :?, and the path.
var quotedVar = regexp.MustCompile(`^"\$(?:(` + varName + `)|\{(` + varName + `)(:\?)?\})(/[^"$` + "`" + `\\]*)?"$`)

// bashBuiltins is every bash builtin. Only a builtin can set a variable of
// the shell running the command; any other command runs in a child.
var bashBuiltins = nameSet(".", ":", "[", "alias", "bg", "bind", "break", "builtin", "caller", "cd", "command",
	"compgen", "complete", "compopt", "continue", "declare", "dirs", "disown", "echo", "enable", "eval", "exec",
	"exit", "export", "false", "fc", "fg", "getopts", "hash", "help", "history", "jobs", "kill", "let", "local",
	"logout", "mapfile", "popd", "printf", "pushd", "pwd", "read", "readarray", "readonly", "return", "set",
	"shift", "shopt", "source", "suspend", "test", "times", "trap", "true", "type", "typeset", "ulimit", "umask",
	"unalias", "unset", "wait")

// inertBuiltins are the builtins that set no variable a caller names and
// run no code from a string; every other builtin can (printf -v "$N",
// trap "$C" DEBUG, wait -p "$N").
var inertBuiltins = nameSet(":", "[", "break", "cd", "continue", "dirs", "echo", "exit", "false", "popd", "pushd",
	"pwd", "return", "set", "shift", "test", "true", "type")

// scratchOperand is true for a scratch target, quoted when it expands
// anything, or a quoted scratch variable, with a path after it with no ..
// only when it can't be empty there, since an empty S makes "$S/x" /x.
func (st *state) scratchOperand(w Word) bool { return st.scratchWord(w, true) }

// scratchWord is scratchOperand, and when bareEmpty is false, a bare "$S"
// must not be empty either, as an empty git -C is the cwd.
func (st *state) scratchWord(w Word, bareEmpty bool) bool {
	quoted := strings.HasPrefix(w.Raw, `"`) && strings.HasSuffix(w.Raw, `"`)
	if m := quotedVar.FindStringSubmatch(w.Raw); m != nil && !strings.Contains(m[4], "..") && st.scratchVar(m[1]+m[2], 1) &&
		(m[4] == "" && bareEmpty || st.nonEmpty(m[1]+m[2], m[3] != "", st.at)) {
		path, fresh := st.varPath(m[1]+m[2], 1)
		return fresh || path != "" && st.onDisk(path+m[4], false)
	}
	v := st.pinWords([]Word{w})[0].Value
	return !varRef.MatchString(v) && (quoted || !strings.Contains(v, "$")) && st.scratchTarget(v, false)
}

// cdPwd is a value that prints a variable's directory with its links
// resolved, $(cd "$T" && pwd -P), or pwd alone; its groups are the name,
// once per form.
var cdPwd = regexp.MustCompile(`^\$\(cd "\$(?:(` + varName + `)|\{(` + varName + `)\})" && pwd(?: -P)?\)$`)

// varPath is the absolute path scratch variable name holds, so the path
// below it gets the symlink check: its value as scratchPath reads it, or,
// up to depth levels, an earlier scratch variable's path and what follows
// it. "" when it can't be read. fresh is true for a mktemp -d dir, which
// holds no link but one the command makes, and rm asks beside ln.
func (st *state) varPath(name string, depth int) (path string, fresh bool) {
	value := st.sure[name]
	if mktempDir(value) {
		return "", true
	}
	if m := cdPwd.FindStringSubmatch(value); m != nil {
		if depth == 0 {
			return "", false
		}
		// cd follows a final link, so its target must be in scratch too.
		path, fresh := st.varPath(m[1]+m[2], depth-1)
		if fresh || path != "" && st.onDisk(path, true) {
			return path, fresh
		}
		return "", false
	}
	if m := quotedVar.FindStringSubmatch(`"` + value + `"`); m != nil {
		if depth == 0 {
			return "", false
		}
		base, fresh := st.varPath(m[1]+m[2], depth-1)
		if fresh || base == "" {
			return "", fresh
		}
		return base + m[4], false
	}
	if p, ok := st.scratchPath(value); ok {
		return p, false
	}
	return "", false
}

// scratchVar is true for a variable the current shell surely sets
// (st.sure) once, outside any loop, and names nowhere else in the command
// (read S, printf -v S, declare S=/ all set it too), to a scratch target,
// a fresh mktemp -d dir, or, up to depth levels, a quoted scratch variable
// set before it, and not empty there, with a path after it (H="$S/x").
func (st *state) scratchVar(name string, depth int) bool {
	if st.inLoop(name) {
		return false
	}
	value, ok := st.sureValue(name, st.at)
	if !ok || !st.setOnly(name) {
		return false
	}
	if mktempDir(value) || st.scratchTarget(value, false) {
		return true
	}
	// D=$(cd "$T" && pwd -P) is T's directory, links resolved, when T can't
	// be empty there (cd "" stays put, and pwd prints the cwd) and T stays
	// in scratch with its last link followed, as cd follows it.
	if m := cdPwd.FindStringSubmatch(value); m != nil {
		inner := m[1] + m[2]
		at, set := st.sureAt[inner]
		if depth == 0 || !set || at >= st.sureAt[name] || !st.scratchVar(inner, depth-1) || !st.nonEmpty(inner, false, st.sureAt[name]) {
			return false
		}
		path, fresh := st.varPath(name, depth)
		return fresh || path != ""
	}
	// value has its quotes removed: H="$S/x", H=$S/x, and H='$S/x' all
	// read $S/x. An assignment doesn't split its value, so the first two
	// are safe; the command's one H= (setOnly) shows which this was.
	m := quotedVar.FindStringSubmatch(`"` + value + `"`)
	if depth == 0 || m == nil || strings.Contains(m[4], "..") {
		return false
	}
	inner := m[1] + m[2]
	return st.setFromEarlier(name, inner) && st.scratchVar(inner, depth-1) && st.nonEmpty(inner, m[3] != "", st.sureAt[name])
}

// nonEmpty is true when scratch variable name can't be empty at segment
// use: used as ${name:?}, which stops the shell, set to a literal path or
// to another scratch variable (scratchVar checked that one), or set by a
// bare assignment whose $(...) failing stops the command first, under an
// earlier set -e when it stands alone (set -e spares a failure left of &&
// or ||), or by an && chain through to use; never after !.
func (st *state) nonEmpty(name string, colonQ bool, use int) bool {
	value := st.sure[name]
	switch {
	case colonQ, !strings.ContainsAny(value, "$`"), quotedVar.MatchString(`"` + value + `"`):
		return true
	case st.exported[name]:
		// export's own status is 0 whatever its $(...) did.
		return false
	}
	set := st.sureAt[name]
	assign := st.segs[set]
	// ! turns a failed $(...) into success, for set -e and && alike.
	if len(assign.Calls) == 0 || inverted(assign) {
		return false
	}
	if standalone(assign) && errexit(st.segs[:set]) {
		return true
	}
	// A failed $(...) fails the assignment, and every && after it skips.
	return use > set && andChain(st.segs, set, use)
}

// errexit is whether segs leave set -e on: the last set that turns it on
// or off, outside any subshell and any part that may not run, turns it on.
func errexit(segs []Segment) bool {
	on := false
	for _, seg := range segs {
		if seg.subshell || seg.conditional {
			continue
		}
		for _, call := range seg.Calls {
			if len(call.Words) == 0 || call.Words[0].Value != "set" || !call.Words[0].literal {
				continue
			}
			words := values(call.Words[1:])
			for i, w := range words {
				if w == "--" {
					break // the rest are positional parameters: set -- -e sets $1
				}
				switch {
				case (w == "-o" || w == "+o") && i+1 < len(words) && words[i+1] == "errexit":
					on = w == "-o"
				case strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "--") && strings.Contains(w, "e"):
					on = true
				case strings.HasPrefix(w, "+") && strings.Contains(w, "e"):
					on = false
				}
			}
		}
	}
	return on
}

// setOnly is true when the command names name only in one assignment and
// in $name references, outside the words and values it passes as written,
// and runs nothing that could set it unseen: no builtin outside
// inertBuiltins but printf without -v, no declaration that sets a name it
// doesn't spell, no command word it computes, and no arithmetic.
func (st *state) setOnly(name string) bool {
	if arithAssigns(st.cmd) {
		return false
	}
	for _, seg := range st.segs {
		for _, call := range seg.Calls {
			if call.declOpaque {
				return false
			}
			i := lead(call.Words)
			if i >= len(call.Words) {
				continue
			}
			// A bare [ is the test builtin, though its name reads as a glob.
			if !call.Words[i].literal && call.Words[i].Raw != "[" {
				if !st.fixedProgram(call.Words[i]) {
					return false
				}
				continue
			}
			name := baseName(call.Words[i].Value)
			// A trap's action is checked as commands of its own (trapAction).
			if name != "trap" && bashBuiltins[name] && !inertBuiltins[name] && !plainPrintf(name, call.Words[i+1:]) {
				return false
			}
		}
	}
	cmd := blankWritten(st.cmd, st.segs)
	q := regexp.QuoteMeta(name)
	count := func(pattern string) int { return len(regexp.MustCompile(pattern).FindAllStringIndex(cmd, -1)) }
	sets := count(`\b` + q + `=`)
	return sets == 1 && count(`\b`+q+`\b`) == sets+count(`\$`+q+`\b|\$\{`+q+`(?::\?)?\}`)
}

// findsOnlyScratch is true when every place find starts from is scratch, as
// rm -r's operands must be, and nothing makes it follow a link out (-L,
// -H, -follow): find -delete removes only what it walks. It fails closed:
// a leading option other than -P, -D x, or -O<n> (BSD's -f, -x, -s, -E,
// -X, -d), a start list read from a file (-files0-from), or no start path
// at all (BSD needs one, GNU takes the cwd) counts as unknown.
func (c *command) findsOnlyScratch() bool {
	args := c.args
	for len(args) > 0 && strings.HasPrefix(args[0].Value, "-") {
		switch v := args[0].Value; {
		case v == "-P", strings.HasPrefix(v, "-O") && len(v) > 2:
		case v == "-D" && len(args) > 1:
			args = args[1:] // its debug options
		default:
			return false
		}
		args = args[1:]
	}
	starts := slices.IndexFunc(args, func(w Word) bool { return findExpression(w.Value) })
	if starts < 0 {
		starts = len(args)
	}
	roots := args[:starts]
	return len(roots) > 0 && !slices.ContainsFunc(c.values(), func(v string) bool {
		return v == "-L" || v == "-H" || v == "-follow" || v == "-f" || v == "-files0-from"
	}) && c.st.allScratch(roots)
}

// findExpression is whether a find word starts its expression: a test,
// action, or operator, such as -name, (, or !.
func findExpression(v string) bool {
	return v == "(" || v == "!" || v == "," || strings.HasPrefix(v, "-")
}

// arithAssign is an assignment operator in an arithmetic expression: =
// alone or after an operator (+=, <<=), ++, or --, but not ==, !=, <=, >=.
var arithAssign = regexp.MustCompile(`(?:^|[^=!<>])=(?:[^=]|$)|<<=|>>=|\+\+|--`)

// arithAssigns is true when an arithmetic expression in cmd, (( )) or
// $(( )), can assign a variable, as (( $N = 0 )) does to a name the command
// never spells; a counter like $((i+1)) assigns nothing. An expression
// with no closing )) runs to the end of cmd.
func arithAssigns(cmd string) bool {
	for i := strings.Index(cmd, "(("); i >= 0; {
		depth, j := 0, i
		for ; j < len(cmd); j++ {
			if cmd[j] == '(' {
				depth++
			} else if cmd[j] == ')' {
				if depth--; depth == 0 {
					break
				}
			}
		}
		if arithAssign.MatchString(cmd[i+2 : min(j, len(cmd))]) {
			return true
		}
		next := strings.Index(cmd[i+2:], "((")
		if next < 0 {
			break
		}
		i += 2 + next
	}
	return false
}

// blankWritten is cmd with spaces over each call word and assigned value
// of segs that the shell passes as written: a name there is text, and no
// call setOnly allows can make it set a variable.
func blankWritten(cmd string, segs []Segment) string {
	b := []byte(cmd)
	blank := func(from, to int) {
		for i := from; i < to; i++ {
			b[i] = ' '
		}
	}
	for _, seg := range segs {
		// A heredoc's commands sit at offsets into its body.
		if seg.src != cmd {
			continue
		}
		for _, call := range seg.Calls {
			for _, w := range call.Words {
				if w.literal {
					blank(w.end-len(w.Raw), w.end)
				}
			}
			for _, span := range call.plainValues {
				blank(span[0], span[1])
			}
		}
	}
	return string(b)
}

// fixedProgram is true for a command word that is one variable the command
// sets once, to a plainWord naming no builtin ($KIT run-checks), or a path
// under a variable ("$V/x", or $V/x with V set so). It runs as a child
// process, which can't set the shell's variables.
func (st *state) fixedProgram(w Word) bool {
	// A path one word long runs a file, never a builtin.
	if m := quotedVar.FindStringSubmatch(w.Raw); m != nil && m[4] != "" {
		return true
	}
	m := programRef.FindStringSubmatch(w.Value)
	if m == nil {
		return false
	}
	name, path := m[1]+m[2], m[3] != ""
	var values []string
	for _, seg := range st.segs {
		if value, ok := assigned(seg)[name]; ok {
			values = append(values, value)
		}
	}
	count := len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`=`).FindAllStringIndex(st.cmd, -1))
	return len(values) == 1 && count == 1 && plainWord.MatchString(values[0]) && (path || !bashBuiltins[baseName(values[0])])
}

// programRef is a command word that is one variable, $V or ${V}, alone or
// followed by a plainWord path.
var programRef = regexp.MustCompile(`^` + nameRef + `(/[A-Za-z0-9_./%:=+@,-]*)?$`)

// plainPrintf is true for printf with no -v, attached (-vNAME) or not,
// which would set a variable, and a first word with no expansion, which
// could turn into one.
func plainPrintf(name string, args []Word) bool {
	return name == "printf" && (len(args) == 0 || !strings.ContainsAny(args[0].Value, "$`")) &&
		!slices.ContainsFunc(args, func(w Word) bool { return strings.HasPrefix(w.Value, "-v") })
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
