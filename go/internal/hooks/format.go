package hooks

import (
	"cmp"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/proc"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/run"
)

// FormatDispatch formats an edited file (PostToolUse for Edit, Write,
// MultiEdit): with the project's fixers (enforce.Edit), else with the
// first on_edit formatter on PATH for it. Then each on_edit note for it
// runs, its output on stderr. It never installs anything and prints
// nothing else: what ran is kept for kit explain stop.
func FormatDispatch(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	path := h.Payload.FilePath()
	if path == "" || !fsx.IsFile(path) || project.IsScratch(h.Paths, path) {
		return nil
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(path))
	cfg := h.Config()
	if err != nil || cfg == nil {
		return nil
	}
	path = filepath.Join(dir, filepath.Base(path))
	root := cmp.Or(git.Toplevel(dir), dir)
	p := enforce.Edit(cfg, enforce.Options{Root: root, CacheDir: h.Paths.CacheDir()}, path)
	deadline := time.Now().Add(formatBudget)
	report := run.Fix(p, deadline)
	chain, notes := enforce.OnEdit(cfg, root, path)
	if !p.Claims() && len(chain) > 0 {
		report += onEdit(chain, path, dir, deadline)
	}
	for _, note := range notes {
		if left := time.Until(deadline); left > 0 && onPath(note) {
			cmd := proc.Command(left, "bash", "-c", fill(note, path))
			cmd.Dir, cmd.Stdout, cmd.Stderr = dir, h.Stderr, h.Stderr
			_ = cmd.Run() // a note only shows what it finds
		}
	}
	if report != "" && !h.DryRun {
		_ = fsx.WriteAtomic(cache.EditReport(h.Paths.CacheDir(), root), []byte(report), 0o600)
	}
	return nil
}

// formatBudget bounds everything one edit runs, fixers, on_edit, and notes
// together, inside post-edit-dispatch's 20s in hooks.json.
const formatBudget = 15 * time.Second

// onEdit formats path with the first of chain whose binary is on PATH, by
// deadline, and says which ran, or that none could.
func onEdit(chain []config.EditCommand, path, dir string, deadline time.Time) string {
	var tried []string
	for _, c := range chain {
		if !onPath(c.Cmd) {
			tried = append(tried, strings.Fields(c.Cmd)[0])
			continue
		}
		left := time.Until(deadline)
		if left <= 0 {
			return "SKIP on_edit: " + c.Cmd + " (out of time)\n"
		}
		runFormatter(c, path, dir, left)
		return "RAN on_edit: " + c.Cmd + "\n"
	}
	return "SKIP on_edit (none on PATH: " + strings.Join(tried, ", ") + ")\n"
}

// onPath is true when cmd's first word is on PATH.
func onPath(cmd string) bool {
	words := strings.Fields(cmd)
	if len(words) == 0 {
		return false
	}
	_, err := exec.LookPath(words[0])
	return err == nil
}

// fill is cmd with {file} as path, one shell word.
func fill(cmd, path string) string { return strings.ReplaceAll(cmd, "{file}", enforce.Quote(path)) }

// runFormatter runs c on path, for at most timeout. A stdout formatter
// reads the file on stdin; its output replaces the file only when it exits
// 0 with output.
func runFormatter(c config.EditCommand, path, dir string, timeout time.Duration) {
	cmd := proc.Command(timeout, "bash", "-c", fill(c.Cmd, path))
	cmd.Dir, cmd.Stderr = dir, io.Discard
	if !c.Stdout {
		_ = cmd.Run() // a formatter that fails leaves the file as it was
		return
	}
	in, err := os.Open(path)
	if err != nil {
		return
	}
	defer in.Close()
	cmd.Stdin = in
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return
	}
	// Write through a symlink (CLAUDE.md -> AGENTS.md), never over it.
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		return
	}
	_ = fsx.WriteAtomic(target, out, info.Mode().Perm())
}
