package bashguard

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/proc"
	"github.com/ku5ic/claude-kit/go/internal/resolve/kittool"
	"github.com/ku5ic/claude-kit/go/internal/run"
	"github.com/ku5ic/claude-kit/go/internal/slop"
)

var (
	// git, any global options, then commit as a word, all in one simple
	// command: the gap crosses no separator or newline but a line continuation.
	commitLine = regexp.MustCompile(`\bgit(?:[ \t]|\\\n)+(?:[^[:space:];&|]+(?:[ \t]|\\\n)+)*commit(?:[^-[:alnum:]_.]|$)`)
)

// CheckCommit is guard-commit: it inspects git commit commands for a staged
// secret (gitleaks), an unchunked wall of text in the body (skills/kit-rules/reference/output.md
// section 1), and AI tells in the message (kit.yml slop). AI attribution is
// guard-attribution's.
func CheckCommit(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	cmd := h.Payload.String(hook.KeyInputCommand)
	segs, unparsed := parseAll(cmd, h.Shell)
	commits := commitsIn(segs, unparsed, h.Home, h.Payload.Cwd())
	if !commits[0].commits(cmd) {
		return nil
	}
	// An expansion other than $HOME or a pinned variable can't be resolved
	// here; most name the cwd's repo ("$PWD", "$(git rev-parse
	// --show-toplevel)").
	pin := pinnedCalls(h, cmd, segs)
	texts := commitTexts(h, cmd, commits, pin)
	ok := approval(h, cmd, segs, unparsed, pin, commits, texts)
	if err := checkMessages(h, cmd, texts); err != nil {
		return err
	}
	if !<-ok {
		h.Ask("skills/kit-rules/reference/workflow.md section 1: the current prompt doesn't ask for a commit, and no earlier reply showed this message; confirm only if the user asked for it", "commit-approval")
	}
	return nil
}

// commitText is one commit's repo and message, as CheckCommit reads them:
// unread says why the message can't be told exactly.
type commitText struct{ repo, msg, unread, subject string }

// commitTexts is each of commits' commitText. A repo pin names comes first.
func commitTexts(h *hook.Hook, cmd string, commits []commit, pin pinnedGit) []commitText {
	texts := make([]commitText, len(commits))
	for i, c := range commits {
		t := &texts[i]
		t.repo = cmp.Or(c.repo, h.Payload.Cwd())
		if p := slices.IndexFunc(pin.calls, func(p commit) bool { return p.start == c.start }); pin.homeKnown && p >= 0 && pin.calls[p].repo != "" {
			t.repo = pin.calls[p].repo
		}
		t.msg, t.unread = message(cmd, c, t.repo, h.Home)
		t.subject = subject(t.msg)
	}
	return texts
}

// approval is whether the user approved the commits, read from the
// transcript while gitleaks and the commit-msg checks run; it touches
// nothing of h. Throwaway repos need none.
func approval(h *hook.Hook, cmd string, segs []Segment, unparsed bool, pin pinnedGit, commits []commit, texts []commitText) <-chan bool {
	ok := make(chan bool, 1)
	if throwaway(h, cmd, segs, unparsed, pin, commits) {
		ok <- true
		return ok
	}
	subjects := make([]string, len(texts))
	for i, t := range texts {
		subjects[i] = t.subject
	}
	go func() {
		// A failed read lets the commit through, as the hook would.
		defer proc.Recover(func(any) { ok <- true })
		ok <- approved(h.Payload.String(hook.KeyTranscriptPath), h.Shell, subjects)
	}()
	return ok
}

// checkMessages scans each repo's staged diff once, the command for a wall
// of text, then each message. The secret scan comes first: it must not
// depend on whether -m used a quoted message.
func checkMessages(h *hook.Hook, cmd string, texts []commitText) error {
	scanned, d := map[string]bool{}, detector(h)
	for i, t := range texts {
		if !scanned[t.repo] {
			scanned[t.repo] = true
			if err := scanStaged(h, t.repo); err != nil {
				return err
			}
		}
		if i == 0 {
			if err := wallOfText(h, cmd); err != nil {
				return err
			}
		}
		if err := checkMessage(h, d, t); err != nil {
			return err
		}
	}
	return nil
}

// checkMessage blocks a message with AI tells, one that can't be read,
// and one the project's commit-msg checks reject.
func checkMessage(h *hook.Hook, d *slop.Detector, t commitText) error {
	if err := scanSlop(h, d, t.msg, "commit", "the commit message"); err != nil {
		return err
	}
	if t.unread != "" {
		return h.Block("can't read the commit message: "+t.unread+"\n"+
			layout.KV("do", "give it in -m, a -m \"$(cat <<'EOF' ...)\" heredoc, -F - with a heredoc, or a file written in an earlier call"), "unread-text")
	}
	if failed := commitMsg(h, t.msg, t.repo); failed != "" {
		return h.Block("the project's commit-msg checks reject this message\n"+failed+"\n"+layout.KV("do", "fix it to their convention"), "commit-msg")
	}
	return nil
}

// wallOfText blocks a commit message paragraph over 4 prose lines. This
// repo passes multi-line messages via a heredoc (-m "$(cat <<'EOF' ...
// EOF)"). A single-line -m has no heredoc and is skipped: short enough that
// a miss is harmless.
func wallOfText(h *hook.Hook, cmd string) error {
	if run := longestProseRun(heredocBody(cmd)); run > 4 {
		reason := fmt.Sprintf("a commit message paragraph runs %d prose lines\n", run) +
			layout.KV("do", "split it into paragraphs of 4 lines or fewer") + "\n" +
			layout.KV("how", "skills/kit-rules/reference/output.md section 1")
		return h.Block(reason, "commit-wall-of-text")
	}
	return nil
}

// scanSlop blocks text, read as surface, when it holds AI tells in a
// slop.block_tiers tier, and logs every hit; where names the text in both.
// A nil d (no kit.yml) fails open.
func scanSlop(h *hook.Hook, d *slop.Detector, text, surface, where string) error {
	if d == nil || strings.TrimSpace(text) == "" {
		return nil
	}
	_, hits := d.Check(slop.Prose(text), surface, func(hit slop.Hit) {
		h.Log(kitlog.Guards, "slop", slop.LogPairs(hit, where)...)
	})
	if len(hits) > 0 {
		return h.Block(slop.Message(where, hits, ""), "slop")
	}
	return nil
}

// detector is h's slop detector; nil without kit.yml.
func detector(h *hook.Hook) *slop.Detector {
	if cfg := h.Config(); cfg != nil {
		return slop.New(cfg)
	}
	return nil
}

// commit is the args and repo of one parsed git commit call in a command;
// found is false when none commits, unparsed true when part of the command
// didn't parse.
type commit struct {
	sub  string // the git subcommand: commit, or another for gitCalls
	args []string
	// repo is the directory it commits in: its git -C directory, against
	// where the cds before it moved, "" when an expansion names it.
	repo            string
	found, unparsed bool
	start           int // source offset of the git call
	// dir is its one git -C word, nil for none; elsewhere is true when
	// something else may pick the repo (several or joined -C, --git-dir,
	// --work-tree, --namespace, -c). seg is the segment it's in, cwd
	// where the cds before it moved, and sure whether the shell surely
	// runs it there (dirs.lost).
	dir       *Word
	elsewhere bool
	seg       int
	cwd       string
	sure      bool
}

// commits reports whether cmd, parsed as c, runs git commit: a parsed call
// whose real command is git with commit as its subcommand. Where part of
// cmd didn't parse, the raw text is matched instead, so a commit whose
// heredoc never closes still counts.
func (c commit) commits(cmd string) bool {
	return c.found || c.unparsed && commitLine.MatchString(cmd)
}

// commitCalls parses cmd, run from cwd, for its commitsIn.
func commitCalls(cmd, home, cwd, shell string) []commit {
	segs, unparsed := parseAll(cmd, shell)
	return commitsIn(segs, unparsed, home, cwd)
}

// commitsIn is every git commit call in segs, run from cwd; with none,
// it's one commit that isn't found.
func commitsIn(segs []Segment, unparsed bool, home, cwd string) []commit {
	out := slices.DeleteFunc(gitCalls(segs, home, cwd, unparsed, nil), func(c commit) bool { return c.sub != "commit" })
	if len(out) == 0 {
		return []commit{{unparsed: unparsed}}
	}
	return out
}

// gitCalls is every git call of segs, run from cwd, read as a commit is.
func gitCalls(segs []Segment, home, cwd string, unparsed bool, pin pinFunc) []commit {
	var out []commit
	for w := range walkPinned(segs, home, cwd, pin) {
		if w.name != "git" {
			continue
		}
		if sub, args, dir := classify.GitSubcommand(values(w.call.Words[w.i+1:])); sub != "" {
			repo := resolveDir(home, w.cwd, dir)
			if strings.ContainsAny(repo, "$`") {
				repo = ""
			}
			c := commit{sub: sub, args: args, repo: repo, found: true, unparsed: unparsed, start: w.call.start, cwd: w.cwd, sure: w.sure}
			// Read before the subcommand, since git commit -C reuses a message.
			words := w.call.Words[w.i+1:]
			for i := 0; i < len(words) && words[i].Value != sub; i++ {
				switch v := words[i].Value; {
				case v == "-C" && i+1 < len(words) && c.dir == nil:
					c.dir = &words[i+1]
					i++
				case strings.HasPrefix(v, "-C"), v == "-c", strings.HasPrefix(v, "--git-dir"),
					strings.HasPrefix(v, "--work-tree"), strings.HasPrefix(v, "--namespace"):
					c.elsewhere = true
				}
			}
			c.seg = slices.IndexFunc(segs, func(s Segment) bool {
				return slices.ContainsFunc(s.Calls, func(call Call) bool { return call.start == w.call.start })
			})
			out = append(out, c)
		}
	}
	return out
}

// commitMsgBudget bounds the project's commit-msg checks, inside
// bash-dispatch's timeout in hooks.json, gitleaks' share apart.
const commitMsgBudget = 12 * time.Second

// commitMsg runs the commit-msg enforcement of repo on msg, from a temp
// file, and returns what failed; "" when it passes or the project has
// none.
func commitMsg(h *hook.Hook, msg, repo string) string {
	root, cfg := git.Toplevel(repo), h.Config()
	if root == "" || cfg == nil {
		return ""
	}
	tmp, err := os.CreateTemp("", "kit-commit-msg-*")
	if err != nil {
		return ""
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	_, err = tmp.WriteString(msg)
	if cerr := tmp.Close(); err != nil || cerr != nil {
		return ""
	}
	p := enforce.CommitMsg(cfg, enforce.Options{Root: root, CacheDir: h.Paths.CacheDir(), Home: h.Home}, tmp.Name())
	return run.Commit(p, commitMsgBudget)
}

// scanStaged runs gitleaks on repo's staged diff. Its exit codes: 0 clean, 1 a
// leak, anything else an operational error that fails open with a notice.
func scanStaged(h *hook.Hook, repo string) error {
	gitleaks, err := kittool.Path("gitleaks")
	if err != nil {
		return nil
	}
	// gitleaks exits 1, a leak's code, on a path it can't stat. A missing
	// repo dir has nothing staged: the command makes it, or git fails.
	if !fsx.IsDir(repo) {
		return nil
	}
	err = proc.Command(proc.Quick, gitleaks, "git", "--staged", "--no-banner", "--redact", "--log-level", "error", repo).Run()
	if err == nil {
		return nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		if exit.ExitCode() == 1 {
			return h.Block("gitleaks flagged a secret in the staged diff", "staged-secret")
		}
		fmt.Fprintf(h.Stderr, "%s: gitleaks exited %d (not a leak signal); skipping scan\n", h.Name, exit.ExitCode())
		return nil
	}
	fmt.Fprintf(h.Stderr, "%s: gitleaks failed (%v); skipping scan\n", h.Name, err)
	return nil
}
