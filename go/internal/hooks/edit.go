// Package hooks implements each Claude Code hook as a hook.Check.
package hooks

import (
	"bufio"
	"cmp"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/stackctx"
)

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
	// Without kit.yml nothing is configured: nothing matches, every check fails open.
	cfg := cmp.Or(h.Config(), &config.Config{})
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
		h.Decide("ask", "this is a CI workflow, which changes what runs on every push; confirm the change")
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
	// transcript.EditTools minus NotebookEdit: Payload.FilePath reads only
	// file_path, and NotebookEdit sends notebook_path.
	switch h.Payload.String("tool_name") {
	case "Edit", "Write", "MultiEdit":
	default:
		return nil
	}
	path := h.Payload.FilePath()
	if path == "" || project.IsScratch(h.Paths, path) {
		return nil
	}
	session := h.Payload.SessionID()
	cfg := h.Config()
	if h.Paths.SessionFile(config.SkillsLoaded, session) == "" || cfg == nil {
		return nil
	}

	required := stackctx.FileMapSkills(cfg.SkillFileMap, path)

	marker := func(skill string) string { return h.Paths.SessionFile(config.SkillsLoaded, session, skill) }
	var toCheck []string
	for _, skill := range required {
		if _, err := os.Stat(marker(skill)); err != nil {
			toCheck = append(toCheck, skill)
		}
	}
	if len(toCheck) == 0 {
		return nil
	}

	// A missing or unreadable log fails open rather than block on uncertainty.
	loaded, err := loadedSkills(h.Paths.LogFile(hook.SkillsLog), session)
	if err != nil {
		return nil
	}
	_ = os.MkdirAll(filepath.Join(h.Paths.CacheDir(), config.SkillsLoaded), 0o755)

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
			if f, err := os.Create(marker(skill)); err == nil {
				f.Close()
			}
		} else {
			missing = append(missing, skill)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return h.Block("This edit touches "+path+". Load the following skills via the Skill tool first, then retry the edit: "+strings.Join(missing, ", "), "skills-gate")
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
	// A Reader, not a Scanner: one oversized line must not end the read.
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF && len(line) == 0 {
			return loaded, nil
		}
		if err != nil && err != io.EOF {
			return nil, err
		}
		var entry hook.Entry
		if json.Unmarshal(line, &entry) != nil {
			continue
		}
		if entry.SessionID != nil && *entry.SessionID == session && entry.SkillFile != nil && !entry.Surfaced() {
			loaded[*entry.SkillFile] = true
			// A plugin's skills log as <plugin>:<skill>; kit.yml maps bare names.
			if _, skill, ok := strings.Cut(*entry.SkillFile, ":"); ok && !strings.Contains(skill, "/") {
				loaded[skill] = true
			}
		}
	}
}

// GuardDispatch runs guard-edit's and, with CLAUDE_GUARD_SKILLS=1,
// guard-skills' checks against one payload read, each isolated so one
// failing open never skips the other. The skills gate is opt-in: blocking
// edits until a patterns skill loads is a personal policy, not a default.
func GuardDispatch(h *hook.Hook) int {
	checks := []hook.NamedCheck{{Name: "guard-edit", Check: GuardEdit}}
	if stackctx.SkillsEnforced() {
		checks = append(checks, hook.NamedCheck{Name: "guard-skills", Check: GuardSkills})
	}
	return hook.Run(h, checks...)
}
