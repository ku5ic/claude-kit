package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// rulePartLimit keeps each part under Claude Code's 10,000-character cap on
// one hook's context: past it, Claude sees only a 2,000-character preview.
const rulePartLimit = 9500

// ruleParts packs rules/*.md, whole files in name order, into parts of at
// most rulePartLimit characters each, wrapper included. A single file over
// the limit still gets a part of its own, which tests then flag.
func ruleParts(paths config.Paths) []string {
	files, _ := filepath.Glob(filepath.Join(paths.Root, "rules", "*.md"))
	var bodies []string
	var body strings.Builder
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		if body.Len() > 0 && body.Len()+len(data)+1 > rulePartLimit-ruleWrapperSize {
			bodies = append(bodies, body.String())
			body.Reset()
		}
		body.WriteString("\n")
		body.Write(data)
	}
	if body.Len() > 0 {
		bodies = append(bodies, body.String())
	}
	parts := make([]string, len(bodies))
	for i, b := range bodies {
		parts[i] = fmt.Sprintf("<claude-kit-rules part=\"%d of %d\">\n"+
			"claude-kit rules, part %d of %d. Every part applies to every reply and every task, like CLAUDE.md instructions.\n"+
			"%s</claude-kit-rules>\n", i+1, len(bodies), i+1, len(bodies), b)
	}
	return parts
}

// ruleWrapperSize bounds what RuleParts adds around a part's files.
const ruleWrapperSize = 200

// InjectRules is the SessionStart and SubagentStart hook that ships the
// kit's rules, one part per hooks.json entry (`kit hook inject-rules N`):
// a plugin can't ship rules files, and hook context also reaches subagents
// and eval runs. A part number past the last part prints nothing.
func InjectRules(h *hook.Hook) error {
	if len(h.Args) == 0 {
		return fmt.Errorf("inject-rules: missing part number")
	}
	n, err := strconv.Atoi(h.Args[0])
	if err != nil {
		return fmt.Errorf("inject-rules: part %q: %w", h.Args[0], err)
	}
	parts := ruleParts(h.Paths)
	if n < 1 || n > len(parts) {
		return nil
	}
	if h.Payload.String("hook_event_name") != "SubagentStart" {
		fmt.Fprint(h.Stdout, parts[n-1])
		return nil
	}
	type specific struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	}
	hook.WriteJSON(h.Stdout, struct {
		HookSpecificOutput specific `json:"hookSpecificOutput"`
	}{specific{"SubagentStart", parts[n-1]}})
	return nil
}

// leftoverRulesLink is the directory under <home>/rules that still holds the
// kit's rules from an install-rules.sh install, "" when there is none: the
// kit's own rules dir, or a copy holding every rule file it has (the
// marketplace clone a link points at is never the plugin cache the hooks
// run from). Left in place, the rules would load twice.
func leftoverRulesLink(paths config.Paths) string {
	kitRules, err := filepath.EvalSymlinks(filepath.Join(paths.Root, "rules"))
	if err != nil {
		return ""
	}
	ruleFiles, _ := filepath.Glob(filepath.Join(kitRules, "*.md"))
	entries, _ := os.ReadDir(filepath.Join(paths.Home, "rules"))
	for _, entry := range entries {
		dir := filepath.Join(paths.Home, "rules", entry.Name())
		if !project.IsDir(dir) {
			continue
		}
		if physical, err := filepath.EvalSymlinks(dir); err == nil && physical == kitRules {
			return dir
		}
		complete := len(ruleFiles) > 0
		for _, rule := range ruleFiles {
			if !project.IsFile(filepath.Join(dir, filepath.Base(rule))) {
				complete = false
				break
			}
		}
		if complete {
			return dir
		}
	}
	return ""
}
