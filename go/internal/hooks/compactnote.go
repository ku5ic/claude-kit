package hooks

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// contextRoom is how much one hook may print for Claude: under the 10,000
// characters Claude Code takes (code.claude.com/docs/en/hooks), less room
// for the label Print adds. It's counted in bytes, never fewer than
// characters.
const contextRoom = 9_900

// CompactNote is the PreCompact hook: its stdout becomes instructions for
// the compaction's summary. It never blocks, since that would stop the
// compaction.
func CompactNote(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return nil
	}
	h.Print(compactNote(h, "Keep this in the summary, word for word.", contextRoom))
	return nil
}

// compactNote is what a compaction must not lose, opened by intro and cut
// to room bytes: the active plan and its first open step, the repo's last
// Stop report, and the files the session edited. The files are cut first,
// then the report, and the plan only when it alone doesn't fit. "" when
// no line fits or there's nothing to say.
func compactNote(h *hook.Hook, intro string, room int) string {
	const end = "</compaction-note>\n"
	cwd := h.Payload.Cwd()
	root := git.Toplevel(cwd)
	// A transcript that can't be read leaves the plan and the report.
	entries, _ := transcript.Load(h.Payload.String(hook.KeyTranscriptPath))
	plan, files := planLines(h, cwd), editedFiles(h, entries, root)
	title, report := stopReport(h, entries, root)
	if plan == "" && report == nil && files == nil {
		return ""
	}
	start := "<compaction-note>\n" + intro + "\n"
	room -= len(end)
	note := start + plan
	if len(note) > room {
		note = start
	}
	note += fitLines(title, report, room-len(note))
	note += fitLines("files this session edited:", files, room-len(note))
	if note == start {
		return ""
	}
	return note + end
}

// planLines names the session's active plan and its first open step, ""
// when there's neither.
func planLines(h *hook.Hook, cwd string) string {
	path, err := os.ReadFile(planFile(h, cache.PlanActive, cwd))
	if err != nil {
		return ""
	}
	plan, err := os.ReadFile(string(path))
	if err != nil {
		return ""
	}
	list := steps(plan)
	open := openStep.FindIndex(list)
	if open == nil {
		return ""
	}
	step, _, _ := strings.Cut(string(list[open[1]:]), "\n")
	return fmt.Sprintf("plan: %s\nnext step: %s\n", path, layout.Clip(strings.TrimSpace(step)))
}

// stopReport is the lines of root's last Stop report and the title they go
// under; none when it was written before the session began.
func stopReport(h *hook.Hook, entries []transcript.Entry, root string) (title string, lines []string) {
	path := cache.StopReport(h.Paths.CacheDir(), root)
	info, err := os.Stat(path)
	if root == "" || err != nil || info.ModTime().Before(startedAt(entries)) {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if text := strings.TrimSpace(string(data)); err == nil && text != "" {
		lines = strings.Split(text, "\n")
	}
	return "last Stop checks, at " + info.ModTime().Format("2006-01-02 15:04") + ":", lines
}

// startedAt is the first time entries hold, zero when none does.
func startedAt(entries []transcript.Entry) time.Time {
	for _, e := range entries {
		if at, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
			return at
		}
	}
	return time.Time{}
}

// editedFiles is every file the session's write tools targeted or its Bash
// commands changed, sorted, relative to root inside it.
func editedFiles(h *hook.Hook, entries []transcript.Entry, root string) []string {
	files := transcript.Edits(entries)
	for _, changed := range bashEdits(h) {
		files = append(files, changed...)
	}
	for i, file := range files {
		files[i] = fsx.Abs(h.Payload.Cwd(), file)
		if root != "" {
			files[i] = fsx.Rel(root, files[i])
		}
	}
	slices.Sort(files)
	return slices.Compact(files)
}

// fitLines is title and the first of lines that fit room bytes, then how
// many didn't; "" when none fits.
func fitLines(title string, lines []string, room int) string {
	const more = "(%d more)\n"
	room -= len(title) + 1 + len(fmt.Sprintf(more, len(lines)))
	n := 0
	for n < len(lines) && len(lines[n]) < room {
		room -= len(lines[n]) + 1
		n++
	}
	if n == 0 {
		return ""
	}
	text := title + "\n" + strings.Join(lines[:n], "\n") + "\n"
	if n < len(lines) {
		text += fmt.Sprintf(more, len(lines)-n)
	}
	return text
}
