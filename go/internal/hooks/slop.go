package hooks

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/slop"
)

// GuardSlop denies a Write, Edit, or MultiEdit that adds AI tells in a
// slop.block_tiers tier, naming each with its line and fix, and logs every
// hit. Only the lines the write adds count, less the tells the lines it
// removes held (slop.Carried). In scratch it reads Markdown
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
	surface := "file"
	if h.Paths.IsScratch(path) {
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		surface = "report"
		if strings.HasPrefix(filepath.Base(path), "pr-") {
			surface = "pr"
		}
	}
	// The path decides these, so a skipped or unread type costs no file
	// read and no regexp compile.
	if slop.Skips(cfg, path) || !slop.Reads(path) {
		return nil
	}
	before, after, ok := written(h.Payload, path)
	if !ok {
		return nil
	}
	segs, allows, _ := slop.Segments(path, before, after)
	for _, a := range allows {
		h.Log(kitlog.Guards, "slop-allow", "path", path, "line", strconv.Itoa(a.Line), "reason", a.Reason)
	}
	d := slop.New(cfg)
	res, hits := d.Check(segs, surface, logHit(h, path))
	if len(hits) > 0 && before != "" {
		hits = d.Carried(path, before, after, surface, hits)
	}
	if len(hits) > 0 {
		slopDeny(h, path, hits)
		return nil
	}
	if res.Dense {
		h.AddContext("PreToolUse", d.DensityNote(filepath.Base(path), res))
	}
	return nil
}

// slopDenies is how many times a file's edits are denied for slop in a
// session. The next flagged edit goes through, so a false positive can't
// loop.
const slopDenies = 2

// slopDeny denies an edit of path for hits, naming the allow marker, up to
// slopDenies times per file in the session.
func slopDeny(h *hook.Hook, path string, hits []slop.Hit) {
	counter := h.Paths.SessionFile(cache.Slop, h.Payload.SessionID(), "edit", textHash(path)[:16])
	data, _ := os.ReadFile(counter)
	n, _ := strconv.Atoi(string(data))
	if n >= slopDenies {
		h.Log(kitlog.Guards, "slop-capped", "path", path, "hits", strconv.Itoa(len(hits)))
		return
	}
	then := "a line that's right as written takes an `unslop: allow <reason>` comment on the line above"
	if h.Deny(slop.Message(filepath.Base(path)+" (new lines)", hits, then), "slop") && counter != "" && !h.DryRun {
		_ = fsx.WriteAtomic(counter, []byte(strconv.Itoa(n+1)), 0o600)
	}
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
	switch p.String(hook.KeyToolName) {
	case "Write":
		return before, p.String(hook.KeyInputContent), true
	case bashEditTool:
		// The command already ran: the file holds after, and the snapshot
		// before it gave before.
		return p.String(hook.KeyInputBefore), p.String(hook.KeyInputContent), true
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
