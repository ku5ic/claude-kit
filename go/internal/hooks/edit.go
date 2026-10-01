// Package hooks implements each Claude Code hook as a hook.Check.
package hooks

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
)

// cfgOrEmpty is the loaded config, or an empty one when kit.yml can't load:
// with nothing configured, nothing matches and every check fails open.
func cfgOrEmpty(h *hook.Hook) *config.Config {
	if cfg := h.Config(); cfg != nil {
		return cfg
	}
	return &config.Config{}
}

var (
	gitDir      = regexp.MustCompile(`/\.git/`)
	ciWorkflows = regexp.MustCompile(`\.github/workflows/.*\.ya?ml$`)
)

// GuardEdit blocks reads and writes of credential files and writes to other
// risky paths, regardless of permission rules. Plugins can't ship
// settings.json deny rules, so for a plugin install this is the only
// credential-read protection. PreToolUse for Read, Edit, Write, MultiEdit.
func GuardEdit(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	path := h.Payload.FilePath()
	if path == "" {
		return nil
	}
	h.SetContext("Path: " + path)
	cfg := cfgOrEmpty(h)
	tool := h.Payload.String("tool_name")

	if guard.IsSensitive(cfg, path) {
		reason, rule := "writing a credential or key file is not permitted", "sensitive-write"
		if tool == "Read" {
			reason, rule = "reading a credential or key file is not permitted", "sensitive-read"
		}
		if err := h.Block(reason, rule); err != nil {
			return err
		}
	}

	// Everything below guards writes only.
	if tool == "Read" {
		return nil
	}
	if guard.IsGuardedLockfile(cfg, path) {
		if err := h.Block("lockfile edit. Use the package manager.", "lockfile-edit"); err != nil {
			return err
		}
	}
	if gitDir.MatchString(path) {
		if err := h.Block("edit inside .git/", "git-dir-edit"); err != nil {
			return err
		}
	}
	if guard.IsRCFile(cfg, path) {
		if err := h.Block("direct edit to a shell rc file. Use the dotfiles repo.", "rc-edit"); err != nil {
			return err
		}
	}
	if ciWorkflows.MatchString(path) {
		fmt.Fprintf(h.Stderr, "guard-edit: editing CI workflow %s\n", path)
	}
	if guard.IsOverlay(h.Paths, path) {
		h.Decide("ask", "this is the claude-kit overlay, which can switch the kit's own guards off; confirm the change")
	}
	return nil
}

// GuardSkills blocks edits of mapped file types until the patterns skill
// skill_file_map requires is loaded this session: one extra round trip per
// skill set per session, by design. Reads are never gated.
func GuardSkills(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	switch h.Payload.String("tool_name") {
	case "Edit", "Write", "MultiEdit":
	default:
		return nil
	}
	path := h.Payload.FilePath()
	if path == "" || strings.Contains(path, "/.claude/scratch/") || strings.Contains(path, "/scratch/") {
		return nil
	}
	session := h.Payload.String("session_id")
	cfg := h.Config()
	if session == "" || cfg == nil {
		return nil
	}

	var required []string
	base := filepath.Base(path)
	for _, rule := range cfg.SkillFileMap {
		target := ""
		switch rule.On {
		case "basename":
			target = base
		case "path":
			target = path
		default:
			continue
		}
		if !slices.ContainsFunc(rule.Globs, func(g string) bool { return guard.Glob(g, target) }) {
			continue
		}
		for _, skill := range rule.Skills {
			if skill != "" && !slices.Contains(required, skill) {
				required = append(required, skill)
			}
		}
	}

	cacheDir := filepath.Join(h.Paths.CacheDir(), "skills-loaded")
	var toCheck []string
	for _, skill := range required {
		if _, err := os.Stat(filepath.Join(cacheDir, session+"-"+skill)); err != nil {
			toCheck = append(toCheck, skill)
		}
	}
	if len(toCheck) == 0 {
		return nil
	}

	// A missing or unreadable log fails open rather than block on uncertainty.
	loaded, err := loadedSkills(filepath.Join(h.Paths.LogDir(), "skills.jsonl"), session)
	if err != nil {
		return nil
	}
	os.MkdirAll(cacheDir, 0o755)

	var missing []string
	for _, skill := range toCheck {
		// An exact skill_file (the Skill tool), or a logged path holding
		// /skills/<name>/ (a Read of its SKILL.md).
		found := loaded[skill]
		for file := range loaded {
			if found {
				break
			}
			found = strings.Contains(file, "/skills/"+skill+"/")
		}
		if found {
			if f, err := os.Create(filepath.Join(cacheDir, session+"-"+skill)); err == nil {
				f.Close()
			}
		} else {
			missing = append(missing, skill)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &hook.Blocked{
		Reason: "This edit touches " + path + ". Load the following skills via the Skill tool first, then retry the edit: " + strings.Join(missing, ", "),
		Rule:   "skills-gate",
	}
}

// loadedSkills streams skills.jsonl once for session's skill_file values,
// skipping inject-context's required-skill and suggested-skill markers:
// those mean "surfaced", not "loaded".
func loadedSkills(logPath, session string) (map[string]bool, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	loaded := map[string]bool{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var entry struct {
			SessionID string  `json:"session_id"`
			SkillFile *string `json:"skill_file"`
			Event     string  `json:"event"`
		}
		if json.Unmarshal(scanner.Bytes(), &entry) != nil {
			continue
		}
		if entry.SessionID == session && entry.SkillFile != nil &&
			entry.Event != "required-skill" && entry.Event != "suggested-skill" {
			loaded[*entry.SkillFile] = true
			// A plugin's skills log as <plugin>:<skill>; kit.yml maps bare names.
			if _, skill, ok := strings.Cut(*entry.SkillFile, ":"); ok && !strings.Contains(skill, "/") {
				loaded[skill] = true
			}
		}
	}
	return loaded, nil
}

// GuardDispatch runs guard-edit's and, with CLAUDE_GUARD_SKILLS=1,
// guard-skills' checks against one payload read, each isolated so one
// failing open never skips the other. The skills gate is opt-in: blocking
// edits until a patterns skill loads is a personal policy, not a default.
func GuardDispatch(h *hook.Hook) int {
	checks := []hook.NamedCheck{{Name: "guard-edit.sh", Check: GuardEdit}}
	if os.Getenv("CLAUDE_GUARD_SKILLS") == "1" {
		checks = append(checks, hook.NamedCheck{Name: "guard-skills.sh", Check: GuardSkills})
	}
	return hook.Run(h, checks...)
}
