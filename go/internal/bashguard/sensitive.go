package bashguard

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/guard"
)

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
// sourceOption is how many words after option v its value takes, and
// whether v names the target directory. GNU takes any unambiguous long
// prefix (--target, --suf). In a short cluster, the first value-taking
// letter takes the rest of the word, or the next word when it's last.
func sourceOption(v string) (skip int, target bool) {
	if strings.HasPrefix(v, "--") {
		name, _, attached := strings.Cut(v, "=")
		target = len(name) > 2 && strings.HasPrefix("--target-directory", name)
		takes := target || len(name) > 2 && strings.HasPrefix("--suffix", name)
		return boolInt(!attached && takes), target
	}
	at := strings.IndexAny(v, "tS")
	if at <= 0 {
		return 0, false
	}
	return boolInt(at == len(v)-1), v[at] == 't'
}

// boolInt is 1 for true, 0 for false.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func sources(values []string) []string {
	var ops []string
	targeted := false
	for i := 0; i < len(values); i++ {
		v := values[i]
		switch {
		case v == "--":
			ops = append(ops, values[i+1:]...)
			i = len(values)
		case strings.HasPrefix(v, "-"):
			skip, target := sourceOption(v)
			targeted, i = targeted || target, i+skip
		default:
			ops = append(ops, v)
		}
	}
	if !targeted && len(ops) > 1 {
		ops = ops[:len(ops)-1]
	}
	return ops
}
