package hooks

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/slop"
)

// GuardSlop denies a Write, Edit, or MultiEdit that adds AI tells in a
// slop.block_tiers tier, naming each with its line and fix, and logs the
// rest. Only the lines the write adds count. In scratch it reads Markdown
// only: reports and PR drafts are written there, logs and downloads too.
// PreToolUse for Edit, Write, MultiEdit.
func GuardSlop(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	path, cfg := h.Payload.FilePath(), h.Config()
	if path == "" || cfg == nil {
		return nil
	}
	d := slop.New(cfg)
	surface := "file"
	if project.IsScratch(h.Paths, path) {
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		surface = "report"
		if strings.HasPrefix(filepath.Base(path), "pr-") {
			surface = "pr"
		}
	}
	if d.Skips(path) {
		return nil
	}
	before, after, ok := written(h.Payload, path)
	if !ok {
		return nil
	}
	segs, allows, ok := slop.Segments(path, before, after)
	if !ok {
		return nil
	}
	for _, a := range allows {
		h.Log(kitlog.Guards, "slop-allow", "path", path, "line", strconv.Itoa(a.Line), "reason", a.Reason)
	}
	res := d.Scan(segs, surface)
	for _, hit := range d.Unblocked(res) {
		h.Log(kitlog.Guards, "slop", slop.LogPairs(hit, path)...)
	}
	if hits := d.Blocking(res); len(hits) > 0 {
		h.Deny(slop.Message(filepath.Base(path)+" (new lines)", hits), "slop")
		return nil
	}
	if res.Dense {
		h.AddContext("PreToolUse", slop.DensityNote(filepath.Base(path), res))
	}
	return nil
}

// written is path's text before the tool runs and after: Write's content,
// or the file with each edit applied. False when it can't be told: the file
// unreadable, or an edit whose old_string isn't there (the tool fails on
// its own then).
func written(p *hook.Payload, path string) (before, after string, ok bool) {
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		before = string(data)
	case !errors.Is(err, fs.ErrNotExist):
		return "", "", false
	}
	switch p.String("tool_name") {
	case "Write":
		return before, p.String("tool_input.content"), true
	case "Edit", "MultiEdit":
		after = before
		for _, e := range p.Edits() {
			switch {
			case e.Old == "" && after == "":
				after = e.New
			case !strings.Contains(after, e.Old) || e.Old == "":
				return "", "", false
			case e.ReplaceAll:
				after = strings.ReplaceAll(after, e.Old, e.New)
			default:
				after = strings.Replace(after, e.Old, e.New, 1)
			}
		}
		return before, after, true
	}
	return "", "", false
}
