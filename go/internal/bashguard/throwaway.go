package bashguard

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// gitEnv is a GIT_ variable, which can name another repo, index, or config.
var gitEnv = regexp.MustCompile(`\bGIT_[A-Z0-9_]+`)

// throwawayCalls are the only commands a throwaway commit's command may
// run: none of them can set what git reads, or write a .git file.
var throwawayCalls = nameSet("git", "cd", "mkdir", "mktemp", "set", "true", ":")

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
// scratch. One the command makes counts when the commit runs only once
// cmd's git init of it worked: a scratch dir with no .git yet, or a
// mktemp -d dir that can't be empty there.
func throwaway(h *hook.Hook, cmd string, commits []commit) bool {
	segs, unparsed := parseAll(cmd)
	if unparsed || gitEnv.MatchString(cmd) || gitEnvSet() || !fixedShape(segs) {
		return false
	}
	calls := gitCalls(segs, h.Home, h.Payload.Cwd(), false)
	for _, c := range commits {
		if c.seg < 0 || c.elsewhere || !lands(h, segs, calls, c) {
			return false
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
// separate git dir or template), and nothing is written but to /dev/null.
func fixedShape(segs []Segment) bool {
	for _, seg := range segs {
		for _, call := range seg.Calls {
			for _, r := range call.Redirs {
				if strings.Contains(r.Op, ">") && r.Target.Value != "/dev/null" && !dupTarget.MatchString(r.Target.Value) {
					return false
				}
			}
			if call.decl != "" {
				return false
			}
			if len(call.Words) == 0 {
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
			}
		}
	}
	return true
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
	if literal && c.repo == "" {
		return false
	}
	if init, ok := initFirst(segs, calls, c); ok {
		if literal {
			// A .git there now may be a worktree's file, whose init
			// reinitializes the repo it points at. A cd into a directory
			// that isn't there fails and leaves the shell where it was;
			// git -C one fails the init itself.
			_, err := os.Lstat(filepath.Join(c.repo, ".git"))
			return project.IsScratch(h.Paths, c.repo) && errors.Is(err, os.ErrNotExist) && (c.dir != nil || fsx.IsDir(c.repo))
		}
		return freshTemp(h, segs, c, init)
	}
	if !literal || !fsx.IsDir(c.repo) {
		return false
	}
	gitDir, err := git.Line(c.repo, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return false
	}
	if real, err := filepath.EvalSymlinks(gitDir); err == nil {
		gitDir = real
	}
	return project.IsScratch(h.Paths, gitDir)
}

// initFirst is the segment of a git init of c's repo, named as c names
// it, that c runs only after: c is &&-chained to it, or it stands alone
// under set -e.
func initFirst(segs []Segment, calls []commit, c commit) (int, bool) {
	for _, init := range calls {
		if init.sub != "init" || init.seg < 0 || init.seg >= c.seg || init.elsewhere || !sameDir(init, c) {
			continue
		}
		if andChain(segs, init.seg, c.seg) || standalone(segs[init.seg]) && errexit(segs[:init.seg]) {
			return init.seg, true
		}
	}
	return 0, false
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
	st.begin(segs)
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
	if len(seg.Calls) == 0 {
		return false
	}
	start := seg.Calls[0].start
	if strings.HasPrefix(seg.src[start:], "!") || strings.HasSuffix(strings.TrimRight(seg.src[:start], " \t"), "!") {
		return false
	}
	after := strings.TrimLeft(seg.src[seg.end:], " \t")
	return after == "" || after[0] == ';' && !strings.HasPrefix(after, ";;") || after[0] == '\n'
}
