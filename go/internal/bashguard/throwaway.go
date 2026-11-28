package bashguard

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
)

// repoEnv names one of repoVars anywhere in a command, quoted apart or not.
var repoEnv = regexp.MustCompile(`\b(?:` + strings.Join(repoVars, "|") + `)\b`)

// throwawayCalls are the only commands a throwaway commit's command may
// run: none of them can set what git reads, or write a .git file. printf
// counts only without -v; rm, cp's destination, and a > redirect only
// inside scratch and outside any .git (scratchWrites).
var throwawayCalls = nameSet("git", "cd", "mkdir", "mktemp", "set", "true", ":", "echo", "printf", "rm", "cp")

// throwawayExports are the names a throwaway commit's command may export.
// HOME picks the git config git reads, so throwaway takes it only fresh.
var throwawayExports = nameSet("HOME", "GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_AUTHOR_DATE",
	"GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "GIT_COMMITTER_DATE")

// ScratchRule is what a subagent is told passes unasked in scratch: the
// rm, download, and throwaway commit rules, and the lists they allow.
func ScratchRule() string {
	calls, exports := ThrowawayAllowed()
	return fmt.Sprintf("An rm -r or download here passes unasked, by a literal path or a variable set once to one. "+
		"A test repo goes under this path, never $TMPDIR, and its commit passes unasked when the command names it with git -C <dir> on every git call, runs only %s, and exports only %s. "+
		"Its rm, cp, and > write only inside this path and outside any .git. "+
		"A HOME it sets must be a fresh $(mktemp -d), and a repo whose hooks or config run a program still asks. Never source an env file.",
		strings.Join(calls, ", "), strings.Join(exports, ", "))
}

// ThrowawayAllowed is what a throwaway commit's command may run, git as
// each of its subcommands, and the names it may export, each sorted.
func ThrowawayAllowed() (calls, exports []string) {
	for name := range throwawayCalls {
		if name != "git" {
			calls = append(calls, name)
		}
	}
	for sub := range throwawayGit {
		calls = append(calls, "git "+sub)
	}
	slices.Sort(calls)
	return calls, slices.Sorted(maps.Keys(throwawayExports))
}

// shellSpecial are the names the shell itself reads when set, exported or
// not: which program runs, where a relative cd goes, how words split.
var shellSpecial = nameSet("PATH", "CDPATH", "IFS", "BASH_ENV", "ENV", "GLOBIGNORE")

// throwawayGit are the git subcommands such a command may run; diff and
// log write files with --output.
var throwawayGit = nameSet("init", "add", "commit")

// dupTarget is a redirect target that duplicates or closes a descriptor,
// as the parser writes it (>&2 is "&2").
var dupTarget = regexp.MustCompile(`^&[0-9-]$`)

// setWord is a set word that changes no variable: -e, -u, -x, -o, and
// their names.
var setWord = regexp.MustCompile(`^([-+][euxo]+|pipefail|errexit|nounset|xtrace)$`)

// throwaway is true when every commit in cmd lands in a throwaway repo by
// git's own answer, and cmd can't point git elsewhere before it runs. A
// repo that exists now counts when git rev-parse puts its git dir under
// scratch and nothing in it runs a program. One the command makes counts
// when the commit runs only once cmd's git init of it worked: a scratch
// dir with no .git yet, or a mktemp -d dir that can't be empty there.
func throwaway(h *hook.Hook, cmd string, segs []Segment, unparsed bool, pin pinnedGit, commits []commit) bool {
	if unparsed || repoEnv.MatchString(cmd) || gitEnvSet() || !fixedShape(segs) || !plainAssigns(segs) || !scratchWrites(h, cmd, segs) {
		return false
	}
	if !pin.homeKnown || !pin.freshHome {
		return false
	}
	calls := pin.calls
	for _, c := range commits {
		i := slices.IndexFunc(calls, func(p commit) bool { return p.start == c.start && p.seg >= 0 })
		if i < 0 || calls[i].elsewhere || !lands(h, segs, calls, calls[i]) {
			return false
		}
	}
	return true
}

// pinnedGit is a command's git calls, each word read as the command
// leaves it there: a pinned variable as its path, ~ with the HOME set
// before it. homeKnown is false when the command sets a HOME the guard
// can't follow: neither pinned, nor a fresh mktemp -d dir no ~ in a git
// call reads. freshHome is true when it sets no HOME or only such a fresh
// one, which holds no git config the guard hasn't seen.
type pinnedGit struct {
	calls                []commit
	homeKnown, freshHome bool
}

// pinnedCalls is cmd's pinnedGit, cmd parsed as segs.
func pinnedCalls(h *hook.Hook, cmd string, segs []Segment) pinnedGit {
	st := &state{dirs: dirs{home: h.Home, cwd: h.Payload.Cwd()}, h: h}
	st.begin(cmd, segs)
	next, tilde := 0, false
	calls := gitCalls(segs, h.Home, h.Payload.Cwd(), false, func(at int, words []Word) []Word {
		for ; next < at; next++ {
			st.track(next, segs[next])
		}
		st.at = at
		tilde = tilde || slices.ContainsFunc(words, func(w Word) bool { return strings.HasPrefix(w.Raw, "~") })
		return tildeHome(st.pinWords(words), st.pinnedHome())
	})
	if !setsHome(segs) {
		return pinnedGit{calls, true, true}
	}
	_, pinned := st.pinned("HOME", 2)
	_, fresh := st.varPath("HOME", 1)
	fresh = fresh && st.scratchVar("HOME", 1)
	return pinnedGit{calls, pinned || fresh && !tilde, fresh && !tilde}
}

// tildeHome expands each word's leading unquoted ~ with home, the HOME the
// command has set by then, as bash does; gitCalls would read the session's.
func tildeHome(words []Word, home string) []Word {
	for i, w := range words {
		if strings.HasPrefix(w.Raw, "~") {
			words[i].Value = guard.ExpandHome(home, w.Value)
		}
	}
	return words
}

// setsHome is true when a call in segs assigns or exports HOME.
func setsHome(segs []Segment) bool {
	for _, seg := range segs {
		for _, call := range seg.Calls {
			if slices.Contains(call.Exports, "HOME") || slices.ContainsFunc(call.Assigns, func(a string) bool { return strings.HasPrefix(a, "HOME=") }) {
				return true
			}
		}
	}
	return false
}

// plainAssigns is true when segs set no name the shell reads itself
// (shellSpecial), and export only throwawayExports: by export or declare
// -x, as a command's own prefix, or by a bare assignment to a name the
// environment already exports.
func plainAssigns(segs []Segment) bool {
	for _, seg := range segs {
		for _, call := range seg.Calls {
			names := slices.Clone(call.Exports)
			for _, a := range call.Assigns {
				name, _, _ := strings.Cut(a, "=")
				if shellSpecial[name] {
					return false
				}
				if len(call.Words) > 0 || os.Getenv(name) != "" {
					names = append(names, name)
				}
			}
			if slices.ContainsFunc(names, func(name string) bool { return !throwawayExports[name] }) {
				return false
			}
		}
	}
	return true
}

// repoVars pick the repo, index, or objects git writes, or set config as
// -c does. A config file's location (GIT_CONFIG_GLOBAL) can't move them.
var repoVars = []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY",
	"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_COMMON_DIR", "GIT_NAMESPACE", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT"}

// gitEnvSet is true when the hook's own environment sets one of repoVars;
// the command inherits it.
func gitEnvSet() bool {
	return slices.ContainsFunc(repoVars, func(name string) bool { return os.Getenv(name) != "" })
}

// fixedShape is true when every call in segs is one of throwawayCalls by
// its literal name, with no wrapper, set changes no variable, git uses
// only -C or --no-pager before one of throwawayGit (init without a
// separate git dir or template), and a redirect alone writes only
// /dev/null; scratchWrites judges the rest of what's written.
func fixedShape(segs []Segment) bool {
	for _, seg := range segs {
		for _, call := range seg.Calls {
			if call.declOpaque {
				return false
			}
			if len(call.Words) == 0 {
				if slices.ContainsFunc(call.Redirs, writesFile) {
					return false
				}
				continue
			}
			name := call.Words[0]
			if !name.literal || !throwawayCalls[name.Value] {
				return false
			}
			args := values(call.Words[1:])
			switch name.Value {
			case "set":
				if slices.ContainsFunc(args, func(w string) bool { return !setWord.MatchString(w) }) {
					return false
				}
			case "git":
				if !plainGit(args) {
					return false
				}
			case "printf":
				if !plainPrintf(name.Value, call.Words[1:]) {
					return false
				}
			}
		}
	}
	return true
}

// writesFile is whether r writes a file: a > redirect to anything but
// /dev/null or another descriptor.
func writesFile(r Redir) bool {
	return strings.Contains(r.Op, ">") && r.Target.Value != "/dev/null" && !dupTarget.MatchString(r.Target.Value)
}

// scratchWrites is true when everything cmd writes or deletes, by a >
// redirect, rm, or cp's destination, is a scratch operand, as rm -r's
// must be, and outside any .git dir, its links resolved: such a write
// can't change what a throwaway commit's git reads or runs.
func scratchWrites(h *hook.Hook, cmd string, segs []Segment) bool {
	st := &state{dirs: dirs{home: h.Home, cwd: h.Payload.Cwd()}, h: h}
	st.begin(cmd, segs)
	next, at := 0, 0
	for w := range walkPinned(segs, h.Home, h.Payload.Cwd(), func(i int, words []Word) []Word {
		for ; next < i; next++ {
			st.track(next, segs[next])
		}
		at = i
		return words
	}) {
		st.at, st.cwd, st.lost = at, w.cwd, !w.sure
		var targets []Word
		for _, r := range w.call.Redirs {
			if writesFile(r) {
				targets = append(targets, r.Target)
			}
		}
		args := w.call.Words[w.i+1:]
		ops := nonOptionsOf(args, func(a Word) string { return a.Value })
		switch w.name {
		case "rm":
			targets = append(targets, ops...)
		case "cp":
			// Only the plain shape, SRC... DEST: a long option can be an
			// abbreviation (--targ=DIR), and a recursive copy of a tree
			// (cp -r payload/. r/) can carry a .git into the repo unnamed.
			if len(ops) < 2 || slices.ContainsFunc(args, func(a Word) bool { return strings.HasPrefix(a.Value, "-") && !plainCpFlag.MatchString(a.Value) }) ||
				slices.ContainsFunc(ops[:len(ops)-1], func(src Word) bool { return st.inGitDir(src) }) {
				return false
			}
			targets = append(targets, ops[len(ops)-1])
		}
		if slices.ContainsFunc(targets, func(t Word) bool { return !st.scratchOperand(t) || st.inGitDir(t) }) {
			return false
		}
	}
	return true
}

// plainCpFlag is a cp option that copies files one to one: no recursion,
// no target directory, no long option, or the -- that ends them.
var plainCpFlag = regexp.MustCompile(`^(?:--|-[fpvniPHLX]+)$`)

// gitComponent is a path with a .git dir in it, in any case, as macOS's
// filesystem matches it.
var gitComponent = regexp.MustCompile(`(?i)(^|/)\.git(/|$)`)

// inGitDir is whether word, a scratch operand, names a path in a .git dir,
// as written or with its links resolved: a quoted variable as the path it
// holds, whose fresh mktemp -d dir has no .git but what follows it.
func (st *state) inGitDir(w Word) bool {
	var p string
	if m := quotedVar.FindStringSubmatch(w.Raw); m != nil {
		base, _ := st.varPath(m[1]+m[2], 1)
		p = base + m[4]
	} else {
		p = st.abs(unquote(st.pinWords([]Word{w})[0].Value))
	}
	return gitComponent.MatchString(p) || filepath.IsAbs(p) && gitComponent.MatchString(fsx.PhysicalPath(filepath.Clean(p)))
}

// initValue are the git init flags that take the next word as their value.
var initValue = nameSet("-b", "--initial-branch", "--object-format", "--ref-format")

// plainGit is true for git args with only -C and --no-pager before one
// of throwawayGit, and no init that puts its git dir or hooks elsewhere,
// or names a directory: init sub makes sub/.git, and a commit from the
// directory above finds whatever repo encloses it.
func plainGit(args []string) bool {
	i := 0
	for i < len(args) && strings.HasPrefix(args[i], "-") {
		switch args[i] {
		case "-C":
			i += 2
		case "--no-pager":
			i++
		default:
			return false
		}
	}
	if i >= len(args) || !throwawayGit[args[i]] {
		return false
	}
	if args[i] != "init" {
		return true
	}
	for j := i + 1; j < len(args); j++ {
		switch w := args[j]; {
		case strings.HasPrefix(w, "--separate-git-dir"), strings.HasPrefix(w, "--template"), !strings.HasPrefix(w, "-"):
			return false
		case initValue[w]:
			j++
		}
	}
	return true
}

// lands is true when commit c of segs lands in a throwaway repo: an
// existing one git puts under scratch, or one calls git init first.
func lands(h *hook.Hook, segs []Segment, calls []commit, c commit) bool {
	literal := c.dir == nil || !strings.ContainsAny(c.dir.Value, "$`")
	if literal && c.repo == "" || !c.placed() {
		return false
	}
	if init, ok := initFirst(segs, calls, c); ok {
		if literal {
			// A .git there now may be a worktree's file, whose init
			// reinitializes the repo it points at. git -C a directory
			// that isn't there fails the init itself.
			_, err := os.Lstat(filepath.Join(c.repo, ".git"))
			return h.Paths.IsScratch(c.repo) && errors.Is(err, os.ErrNotExist)
		}
		return freshTemp(h, segs, c, init)
	}
	if !literal || !fsx.IsDir(c.repo) {
		return false
	}
	gitDir, err := git.GitDir(c.repo)
	if err != nil {
		return false
	}
	if real, err := filepath.EvalSymlinks(gitDir); err == nil {
		gitDir = real
	}
	return h.Paths.IsScratch(gitDir) && runsNothing(c.repo)
}

// plainRepoKeys are the repo config keys, lowercased as git lists them,
// that git init and a commit's identity write and that run no program.
// Any other key could, so the repo isn't throwaway.
var plainRepoKeys = regexp.MustCompile(`^(core\.(repositoryformatversion|filemode|bare|logallrefupdates|ignorecase|precomposeunicode|symlinks)|extensions\.(objectformat|refstorage)|user\.(name|email)|init\.defaultbranch|commit\.gpgsign)$`)

// runsNothing is true when repo, made before this command, has no hook a
// template or a write put there, and its own config, worktree's included,
// holds only plainRepoKeys.
func runsNothing(repo string) bool {
	hooks, err := git.Line(repo, "rev-parse", "--path-format=absolute", "--git-path", "hooks")
	if err != nil {
		return false
	}
	entries, err := os.ReadDir(hooks)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false
	}
	if slices.ContainsFunc(entries, func(e os.DirEntry) bool { return !strings.HasSuffix(e.Name(), ".sample") }) {
		return false
	}
	lines, err := git.Lines(repo, "config", "--includes", "--list", "--name-only", "--show-scope")
	return err == nil && !slices.ContainsFunc(lines, func(line string) bool {
		scope, key, _ := strings.Cut(line, "\t")
		return scope != "system" && scope != "global" && !plainRepoKeys.MatchString(key)
	})
}

// initFirst is the segment of a git init of c's repo, named as c names
// it, that c runs only after: c is &&-chained to it, or it stands alone
// under set -e.
func initFirst(segs []Segment, calls []commit, c commit) (int, bool) {
	for _, init := range calls {
		// A pipeline exits with its last call's status: "git init | true"
		// hides a failed init.
		// ! makes a failed init succeed, so the commit runs anyway.
		if init.sub != "init" || init.seg < 0 || init.seg >= c.seg || init.elsewhere || !init.placed() || !sameDir(init, c) || len(segs[init.seg].Calls) != 1 || inverted(segs[init.seg]) {
			continue
		}
		if andChain(segs, init.seg, c.seg) || standalone(segs[init.seg]) && errexit(segs[:init.seg]) {
			return init.seg, true
		}
	}
	return 0, false
}

// placed is true when c runs where its repo says: the shell surely runs
// it in the cwd it was given, or its -C is absolute.
func (c commit) placed() bool {
	return c.sure || c.dir != nil && filepath.IsAbs(c.dir.Value)
}

// sameDir is true when two git calls name one repo: the same -C word as
// written, or no -C from the same literal directory.
func sameDir(a, b commit) bool {
	if a.dir == nil || b.dir == nil {
		return a.dir == nil && b.dir == nil && a.repo != "" && a.repo == b.repo
	}
	return a.dir.Raw == b.dir.Raw && (a.repo == b.repo || a.repo == "")
}

// freshTemp is true when c's -C is "$t" alone, t a mktemp -d dir the
// current shell surely set and that can't be empty at the init: an
// empty -C is the cwd's repo.
func freshTemp(h *hook.Hook, segs []Segment, c commit, init int) bool {
	m := quotedVar.FindStringSubmatch(c.dir.Raw)
	if m == nil || m[4] != "" {
		return false
	}
	st := &state{dirs: dirs{home: h.Home, cwd: c.cwd}, h: h}
	st.begin(h.Payload.String(hook.KeyInputCommand), segs)
	for i, seg := range segs[:init] {
		st.track(i, seg)
	}
	st.at = init
	return mktempDir(st.sure[m[1]+m[2]]) && st.scratchWord(*c.dir, false)
}

// andChain is true when segment to runs only if segment from succeeded:
// every pipeline between them, outside any subshell, follows an &&.
func andChain(segs []Segment, from, to int) bool {
	prev := segs[from]
	for _, seg := range segs[from+1 : to+1] {
		if seg.subshell {
			continue
		}
		if len(seg.Calls) == 0 || prev.end > seg.Calls[0].start || strings.TrimSpace(prev.src[prev.end:seg.Calls[0].start]) != "&&" {
			return false
		}
		prev = seg
	}
	return true
}

// standalone is true when nothing but ; or a newline follows seg, and no !
// negates it: set -e stops on its failure there and nowhere else.
func standalone(seg Segment) bool {
	if len(seg.Calls) == 0 || inverted(seg) {
		return false
	}
	after := strings.TrimLeft(seg.src[seg.end:], " \t")
	return after == "" || after[0] == ';' && !strings.HasPrefix(after, ";;") || after[0] == '\n'
}

// inverted is true when a ! opens seg's pipeline, inverting its status.
func inverted(seg Segment) bool {
	if len(seg.Calls) == 0 {
		return false
	}
	start := seg.Calls[0].start
	return strings.HasPrefix(seg.src[start:], "!") || strings.HasSuffix(strings.TrimRight(seg.src[:start], " \t"), "!")
}
