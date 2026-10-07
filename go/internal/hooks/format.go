package hooks

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/extract"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/tools"
)

// FormatDispatch formats an edited file with the project's own formatter
// and config, per kit.yml's formatters table. A file two formatters claim (a
// migration in progress) is left byte-identical, and so is one none claims,
// unless a fallback entry (Markdown) resolves.
// Never installs anything. Shell files also get a non-blocking shellcheck
// pass on stderr. PostToolUse for Edit, Write, MultiEdit.
func FormatDispatch(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	path := h.Payload.FilePath()
	if info, err := os.Stat(path); path == "" || err != nil || !info.Mode().IsRegular() {
		return nil
	}
	if project.IsScratch(h.Paths, path) {
		return nil
	}
	base := filepath.Base(path)
	if !strings.Contains(base, ".") {
		return nil
	}
	ext := strings.ToLower(base[strings.LastIndexByte(base, '.')+1:])
	dir, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return nil
	}
	root := project.Toplevel(dir)
	if root == "" {
		root = dir
	}
	cfg := h.Config()
	if cfg == nil {
		return nil
	}

	type hit struct {
		fmt config.Formatter
		res tools.Resolution
	}
	var hits, blocked []hit
	var fallbacks []config.Formatter
	for _, f := range cfg.Formatters {
		if slices.Contains(cfg.DisabledFormatters, f.Name) || !slices.Contains(f.Ext, ext) {
			continue
		}
		if f.Fallback {
			fallbacks = append(fallbacks, f)
			continue
		}
		// Resolving can start a package manager (poetry env info, bundle
		// info), so only a formatter with a signal, or Prettier's own lookup,
		// which needs the binary, pays for it.
		signaled := hasSignal(f, nil, dir, root, path)
		if !signaled && !f.SignalPrettier {
			continue
		}
		res := tools.Resolve(cfg, dir, root, f.Bin, tools.Default)
		switch {
		case signaled:
		case res.Project:
			// The project names the tool (declared, pinned) but it can't
			// run: no fallback may swap another formatter in.
			blocked = append(blocked, hit{f, res})
		case res.Words != nil:
			signaled = hasSignal(f, res.Words, dir, root, path)
		case res.OnPath != "":
			// Any copy can answer Prettier's config lookup, even one the
			// policy won't format with.
			signaled = hasSignal(f, []string{res.OnPath}, dir, root, path)
		}
		if signaled {
			hits = append(hits, hit{f, res})
		}
	}
	if len(hits) == 0 && len(blocked) > 0 {
		hits = blocked[:1]
	}
	if len(hits) == 0 {
		for _, f := range fallbacks {
			if res := tools.Resolve(cfg, dir, root, f.Bin, tools.AnyPath); !res.Missing {
				hits = append(hits, hit{f, res})
				break
			}
		}
	}

	switch {
	case len(hits) > 1:
		var names []string
		for _, h := range hits {
			names = append(names, h.fmt.Name)
		}
		fmt.Fprintf(h.Stderr, "format-dispatch: left %s unformatted; %s are all configured for it here\n", base, strings.Join(names, " "))
	case len(hits) == 1 && hits[0].res.Missing:
		fmt.Fprintf(h.Stderr, "format-dispatch: %s is configured here but not installed; %s left unformatted\n", hits[0].fmt.Name, base)
	case len(hits) == 1 && hits[0].res.Words == nil:
		fmt.Fprintf(h.Stderr, "format-dispatch: %s is configured here but can't run: %s; %s left unformatted\n", hits[0].fmt.Name, hits[0].res.Skip, base)
	case len(hits) == 1:
		runFormatter(hits[0].fmt, hits[0].res.Words, path, dir, h.Stderr)
	}

	if ext == "sh" || ext == "bash" {
		if res := tools.Resolve(cfg, dir, root, "shellcheck", tools.Default); res.Words != nil {
			cmd := exec.Command(res.Words[0], append(res.Words[1:], path)...)
			cmd.Stdout, cmd.Stderr = h.Stderr, h.Stderr
			_ = cmd.Run() // advisory: its findings are on stderr already
		}
	}
	return nil
}

// runFormatter runs f on path with bin filling {bin}, word by word so a
// path with spaces stays one argument. A stdout formatter reads the file on
// stdin; its output replaces the file only when it exits 0 with output.
func runFormatter(f config.Formatter, bin []string, path, dir string, stderr io.Writer) {
	var parts []string
	for _, word := range tools.Fill(strings.Fields(f.Cmd), "{bin}", bin) {
		parts = append(parts, strings.ReplaceAll(word, "{file}", path))
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir, cmd.Stderr = dir, stderr
	if !f.Stdout {
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
	path = target
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(out)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Chmod(tmp.Name(), info.Mode().Perm()) != nil || os.Rename(tmp.Name(), path) != nil {
		os.Remove(tmp.Name())
	}
}

// hasSignal is true when formatter f has a signal for the file: a config
// file by name or a TOML table walking up from dir to root, or Prettier's
// own config lookup, accepted only when what it finds is inside root (it
// also finds a ~/.prettierrc, and every repo would get Prettier).
func hasSignal(f config.Formatter, bin []string, dir, root, path string) bool {
	if len(f.SignalFiles) > 0 && project.FindUp(dir, root, f.SignalFiles...) != "" {
		return true
	}
	if f.SignalTOML != "" {
		file, table, _ := strings.Cut(f.SignalTOML, " ")
		if found := project.FindUp(dir, root, file); found != "" && extract.TOMLHas(found, table) {
			return true
		}
	}
	if f.SignalPrettier && bin != nil {
		cmd := exec.Command(bin[0], append(bin[1:], "--find-config-path", path)...)
		cmd.Dir = dir
		out, err := cmd.Output()
		found := strings.TrimSpace(string(out))
		if err == nil && found != "" {
			if !filepath.IsAbs(found) {
				found = filepath.Join(dir, found)
			}
			return strings.HasPrefix(project.PhysicalPath(found), root+"/")
		}
	}
	return false
}
