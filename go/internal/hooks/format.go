package hooks

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/proc"
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
	if path == "" || !fsx.IsFile(path) || project.IsScratch(h.Paths, path) {
		return nil
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if ext == "" {
		return nil
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(path))
	cfg := h.Config()
	if err != nil || cfg == nil {
		return nil
	}
	root := cmp.Or(project.Toplevel(dir), dir)
	hits, blocked, fallbacks := claims(cfg, ext, dir, root, path)
	also := ""
	if len(hits) == 0 && len(blocked) > 0 {
		hits, blocked = blocked[:1], blocked[1:]
	}
	if len(blocked) > 0 {
		// Can't tell whether it's configured too without running it.
		also = fmt.Sprintf(" (%s is also declared here but can't run: %s)", blocked[0].fmt.Name, blocked[0].res.Skip)
	}
	if len(hits) == 0 {
		for _, f := range fallbacks {
			if res := tools.Resolve(cfg, dir, root, f.Bin, tools.AnyPath); !res.Missing {
				hits = append(hits, claim{f, res})
				break
			}
		}
	}
	apply(hits, also, path, dir, h.Stderr)
	if ext == "sh" || ext == "bash" {
		if res := tools.Resolve(cfg, dir, root, "shellcheck", tools.Default); res.Words != nil {
			cmd := proc.Command(formatTimeout, res.Words[0], append(res.Words[1:], path)...)
			cmd.Stdout, cmd.Stderr = h.Stderr, h.Stderr
			_ = cmd.Run() // advisory: its findings are on stderr already
		}
	}
	return nil
}

// claim is a formatter that claims the file, and how it runs.
type claim struct {
	fmt config.Formatter
	res tools.Resolution
}

// claims sorts the formatters for ext: hits have a signal for the file,
// blocked are ones the project names but that can't run, and fallbacks
// claim a file nothing else does.
func claims(cfg *config.Config, ext, dir, root, path string) (hits, blocked []claim, fallbacks []config.Formatter) {
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
			blocked = append(blocked, claim{f, res})
		case res.Words != nil:
			signaled = hasSignal(f, res.Words, dir, root, path)
		case res.OnPath != "":
			// Any copy can answer Prettier's config lookup, even one the
			// policy won't format with.
			signaled = hasSignal(f, []string{res.OnPath}, dir, root, path)
		}
		if signaled {
			hits = append(hits, claim{f, res})
		}
	}
	return hits, blocked, fallbacks
}

// apply runs the one formatter that claims the file, or says why none ran.
func apply(hits []claim, also, path, dir string, stderr io.Writer) {
	base := filepath.Base(path)
	switch {
	case len(hits) > 1:
		var names []string
		for _, c := range hits {
			names = append(names, c.fmt.Name)
		}
		fmt.Fprintf(stderr, "format-dispatch: left %s unformatted; %s are all configured for it here\n", base, strings.Join(names, " "))
	case len(hits) == 1 && hits[0].res.Missing:
		fmt.Fprintf(stderr, "format-dispatch: %s is configured here but not installed; %s left unformatted%s\n", hits[0].fmt.Name, base, also)
	case len(hits) == 1 && hits[0].res.Words == nil:
		fmt.Fprintf(stderr, "format-dispatch: %s is configured here but can't run: %s; %s left unformatted%s\n", hits[0].fmt.Name, hits[0].res.Skip, base, also)
	case len(hits) == 1:
		runFormatter(hits[0].fmt, hits[0].res.Words, path, dir, stderr)
	}
}

// formatTimeout bounds one formatter run, inside post-edit-dispatch's 20s
// in hooks.json.
const formatTimeout = 15 * time.Second

// runFormatter runs f on path with bin filling {bin}, word by word so a
// path with spaces stays one argument. A stdout formatter reads the file on
// stdin; its output replaces the file only when it exits 0 with output.
func runFormatter(f config.Formatter, bin []string, path, dir string, stderr io.Writer) {
	var parts []string
	for _, word := range tools.Fill(strings.Fields(f.Cmd), "{bin}", bin) {
		parts = append(parts, strings.ReplaceAll(word, "{file}", path))
	}
	cmd := proc.Command(formatTimeout, parts[0], parts[1:]...)
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
	_ = fsx.WriteAtomic(target, out, info.Mode().Perm())
}

// hasSignal is true when formatter f has a signal for the file: a config
// file or TOML table (tools.HasSignal), or Prettier's
// own config lookup, accepted only when what it finds is inside root (it
// also finds a ~/.prettierrc, and every repo would get Prettier).
func hasSignal(f config.Formatter, bin []string, dir, root, path string) bool {
	if _, ok := tools.HasSignal(f.SignalFiles, f.SignalTOML, dir, root); ok {
		return true
	}
	if f.SignalPrettier && bin != nil {
		cmd := proc.Command(proc.Quick, bin[0], append(bin[1:], "--find-config-path", path)...)
		cmd.Dir = dir
		out, err := cmd.Output()
		found := strings.TrimSpace(string(out))
		if err == nil && found != "" {
			if !filepath.IsAbs(found) {
				found = filepath.Join(dir, found)
			}
			return strings.HasPrefix(fsx.PhysicalPath(found), root+"/")
		}
	}
	return false
}
