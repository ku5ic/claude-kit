package hooks

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/hook"
)

var (
	gitCommit    = regexp.MustCompile(`git[[:space:]]+([^[:space:]]+[[:space:]]+)*commit`)
	aiSignature  = regexp.MustCompile(`(?i)Co-Authored-By:[[:space:]]*Claude|Generated[[:space:]]+(by|with)[[:space:]]+Claude|🤖[[:space:]]*Generated`)
	heredocOpen  = regexp.MustCompile(`<<-?['"]?EOF['"]?`)
	messageDQ    = regexp.MustCompile(`(-m|--message=?)[[:space:]]*"[^"]*"`)
	messageSQ    = regexp.MustCompile(`(-m|--message=?)[[:space:]]*'[^']*'`)
	aiTell       = regexp.MustCompile(`(?i)^(feat|fix|chore|refactor|docs|test|perf|build|ci|style)?:?[[:space:]]*(certainly|here is|i have|let me|in this commit|this commit)`)
	frontmatter  = regexp.MustCompile(`^---[[:space:]]*$`)
	nonProseLine = regexp.MustCompile(`^[[:space:]]*([0-9]+[.)]|[-*+][[:space:]]|#{1,6}[[:space:]]|>|\|)`)
)

// GuardCommit inspects git commit commands for AI signatures, a staged
// secret (gitleaks), an unchunked wall of text in the body (rules/output.md
// section 1), and AI-tell phrasing in the subject.
func GuardCommit(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	cmd := h.Payload.String("tool_input.command")
	if !gitCommit.MatchString(cmd) {
		return nil
	}

	// Line by line, as grep: the trailer sits on a line of its own.
	for line := range strings.SplitSeq(cmd, "\n") {
		if aiSignature.MatchString(line) {
			if err := h.Block("AI signature in commit message", "ai-commit-sig"); err != nil {
				return err
			}
			break
		}
	}

	// Before the subject parsing: secret scanning must not depend on
	// whether -m used a quoted message.
	if err := scanStaged(h); err != nil {
		return err
	}

	// This repo passes multi-line messages via a heredoc
	// (-m "$(cat <<'EOF' ... EOF)"). A single-line -m has no heredoc and is
	// skipped: short enough that a miss is harmless.
	if body := heredocBody(cmd); body != "" {
		if run := longestProseRun(body); run > 4 {
			reason := fmt.Sprintf("commit message has an unchunked wall of text (%d consecutive prose lines). rules/output.md section 1: short paragraphs, no dense blocks.", run)
			if err := h.Block(reason, "commit-wall-of-text"); err != nil {
				return err
			}
		}
	}

	subject, _, _ := strings.Cut(quotedMessages(cmd, messageDQ, '"')+quotedMessages(cmd, messageSQ, '\''), "\n")
	if subject != "" && aiTell.MatchString(subject) {
		return h.Block("AI-tell phrasing in commit subject", "ai-commit-tell")
	}
	return nil
}

// scanStaged runs gitleaks on the staged diff. Its exit codes: 0 clean, 1 a
// leak, anything else an operational error that fails open with a notice.
func scanStaged(h *hook.Hook) error {
	if _, err := exec.LookPath("gitleaks"); err != nil {
		return nil
	}
	dir := h.Payload.String("cwd")
	if dir == "" {
		dir = "."
	}
	err := exec.Command("gitleaks", "git", "--staged", "--no-banner", "--redact", "--log-level", "error", dir).Run()
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

// heredocBody is the message between the opener and the closing EOF of the
// heredoc opened on the git commit line (-m "$(cat <<'EOF'", -F - <<'EOF').
// Other heredocs in the command write files, not the message.
func heredocBody(cmd string) string {
	var lines []string
	in := false
	for line := range strings.SplitSeq(cmd, "\n") {
		switch {
		case in && line == "EOF":
			return strings.Join(lines, "\n")
		case in:
			lines = append(lines, line)
		case heredocOpen.MatchString(line) && gitCommit.MatchString(line):
			in = true
		}
	}
	return ""
}

// quotedMessages is `$(grep -oE '<re>' | sed 's/.*<q>([^<q>]*)<q>/\1/')`:
// the quoted text of every -m/--message match, one per line, matched line by
// line, with no trailing newline (command substitution strips it, so the
// double- and single-quoted results concatenate directly).
func quotedMessages(cmd string, re *regexp.Regexp, quote byte) string {
	var out []string
	for line := range strings.SplitSeq(cmd, "\n") {
		for _, match := range re.FindAllString(line, -1) {
			inner := strings.TrimSuffix(match, string(quote))
			out = append(out, inner[strings.LastIndexByte(inner, quote)+1:])
		}
	}
	return strings.Join(out, "\n")
}

// longestProseRun is the longest run of consecutive non-blank lines that
// aren't list items, headings, blockquotes, or table rows, outside fenced
// code and YAML frontmatter: a deterministic stand-in for rules/output.md
// section 1. Frontmatter is skipped because its key: value lines would read
// as a wall, blocking every agent and skill header.
func longestProseRun(text string) int {
	best, run := 0, 0
	inFront, inFence := false, false
	for i, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		switch {
		case i == 0 && frontmatter.MatchString(line):
			inFront = true
		case inFront:
			if frontmatter.MatchString(line) {
				inFront = false
			}
		case strings.HasPrefix(line, "```"):
			inFence = !inFence
		case inFence:
		case strings.TrimSpace(line) == "":
			run = 0
		case nonProseLine.MatchString(line):
			run = 0
		default:
			run++
			best = max(best, run)
		}
	}
	return best
}
