package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/dotfiles/claude-kit/go/internal/config"
	"github.com/ku5ic/dotfiles/claude-kit/go/internal/extract"
	"github.com/ku5ic/dotfiles/claude-kit/go/internal/hook"
	"github.com/ku5ic/dotfiles/claude-kit/go/internal/project"
	"github.com/ku5ic/dotfiles/claude-kit/go/internal/tools"
)

// FormatDispatch formats an edited file with the project's own formatter
// and config, per kit.yml's formatters table. A file no formatter claims, or
// one two formatters claim (a migration in progress), is left byte-identical.
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
	if strings.Contains(path, "/.claude/scratch/") || strings.Contains(path, "/scratch/") {
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
		bin []string // the words that run it, nil when not installed
	}
	var hits []hit
	for _, f := range cfg.Formatters {
		if slices.Contains(cfg.DisabledFormatters, f.Name) || !slices.Contains(f.Ext, ext) {
			continue
		}
		// Resolving can start a package manager (poetry env info, bundle
		// info), so only a formatter with a signal, or Prettier's own lookup,
		// which needs the binary, pays for it.
		signaled := hasSignal(f, nil, dir, root, path)
		if !signaled && !f.SignalPrettier {
			continue
		}
		bin := tools.Resolve(dir, root, f.Bin, false)
		if signaled || hasSignal(f, bin, dir, root, path) {
			hits = append(hits, hit{f, bin})
		}
	}

	switch {
	case len(hits) > 1:
		var names []string
		for _, h := range hits {
			names = append(names, h.fmt.Name)
		}
		fmt.Fprintf(h.Stderr, "format-dispatch: left %s unformatted; %s are all configured for it here\n", base, strings.Join(names, " "))
	case len(hits) == 1 && hits[0].bin == nil:
		fmt.Fprintf(h.Stderr, "format-dispatch: %s is configured here but not installed; %s left unformatted\n", hits[0].fmt.Name, base)
	case len(hits) == 1:
		// Word by word, so a path with spaces stays one argument; {bin} can
		// be several words (yarn run prettier under Yarn PnP).
		var parts []string
		for _, word := range strings.Fields(hits[0].fmt.Cmd) {
			if word == "{bin}" {
				parts = append(parts, hits[0].bin...)
				continue
			}
			parts = append(parts, strings.ReplaceAll(word, "{file}", path))
		}
		cmd := exec.Command(parts[0], parts[1:]...)
		cmd.Dir, cmd.Stderr = dir, h.Stderr
		cmd.Run()
	}

	if ext == "sh" || ext == "bash" {
		if _, err := exec.LookPath("shellcheck"); err == nil {
			cmd := exec.Command("shellcheck", path)
			cmd.Stdout, cmd.Stderr = h.Stderr, h.Stderr
			cmd.Run()
		}
	}
	return nil
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
