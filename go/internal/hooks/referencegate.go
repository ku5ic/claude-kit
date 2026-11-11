package hooks

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
)

// ReferenceGate is the PreToolUse check after rules-gate on every tool: it
// denies a call until the session, or the subagent making it, has read
// every reference the "Before | Read" table of each skill it loaded links,
// naming all of them at once. A read counts until the next compaction. A
// fork counts its parent's loads and reads. A Read of a skill's own files
// passes, so the deny can always be met, and so does an agent Claude Code
// gave no Read tool. A missing skills log fails open.
func ReferenceGate(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return p.Err
	}
	tool := p.String("tool_name")
	if gatePasses(tool) || tool == "Read" && strings.Contains(p.FilePath(), "/skills/") {
		return nil
	}
	agent := p.String("agent_id")
	missing, err := unreadReferences(h, agent)
	if err != nil || len(missing) == 0 {
		return nil
	}
	if start := subagentStart(p); start.Fork {
		if missing, err = unreadReferences(h, ""); err != nil || len(missing) == 0 {
			return nil
		}
	} else if withoutTools(start, "Read") {
		h.Log(kitlog.Guards, "reference-gate-unreadable", "agent_type", p.String("agent_type"))
		return nil
	}
	h.Deny("read "+strings.Join(missing, ", ")+" first, then retry this call. Each is in the table of a skill you loaded, and every tool call stays blocked until they're read, so don't stop to ask", "reference-gate")
	return nil
}

// unreadReferences is the table references of the skills agent ("" for the
// main session) loaded that it hasn't read since the last compaction, as
// absolute paths.
func unreadReferences(h *hook.Hook, agent string) ([]string, error) {
	logPath := h.Paths.LogFile(kitlog.Skills)
	if _, err := os.Stat(logPath); err != nil {
		return nil, err
	}
	var dirs []string
	read := map[string]bool{}
	err := eachSkillEntry(logPath, h.Payload.SessionID(), func(entry kitlog.Entry) {
		switch {
		case entry.Event == kitlog.EventCompacted:
			clear(read)
		case entry.AgentID != agent || !entry.Loads() && entry.Event != kitlog.EventReferenceRead:
		case entry.Event == kitlog.EventReferenceRead:
			if entry.SkillFile != nil {
				read[filepath.Clean(*entry.SkillFile)] = true
			}
		case entry.SkillFile != nil && strings.HasSuffix(*entry.SkillFile, "/SKILL.md"):
			dirs = appendNew(dirs, filepath.Dir(*entry.SkillFile))
		case entry.SkillFile != nil:
			dirs = appendNew(dirs, kitSkillDir(h.Paths.Root, *entry.SkillFile))
		case entry.CommandName != nil:
			dirs = appendNew(dirs, kitSkillDir(h.Paths.Root, strings.TrimPrefix(*entry.CommandName, "/")))
		}
	})
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, dir := range dirs {
		for _, ref := range tableReferences(dir) {
			if path := filepath.Join(dir, "reference", ref); !read[path] {
				missing = append(missing, path)
			}
		}
	}
	return missing, nil
}

func appendNew(list []string, s string) []string {
	if s == "" || slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}

// tableLink is a reference link in a "Before | Read" table row.
var tableLink = regexp.MustCompile(`\]\(reference/([^)/]+\.md)\)`)

// tableReferences is the reference files that skill dir's SKILL.md links
// from its "Before | Read" table and that exist, in order, each once. A
// procedure skill has no such table, and reads its references step by step.
func tableReferences(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return nil
	}
	var refs []string
	inTable := false
	for line := range strings.Lines(string(data)) {
		switch {
		case strings.HasPrefix(line, "| Before "):
			inTable = true
		case inTable && !strings.HasPrefix(line, "|"):
			inTable = false
		case inTable:
			for _, m := range tableLink.FindAllStringSubmatch(line, -1) {
				if _, err := os.Stat(filepath.Join(dir, "reference", m[1])); err == nil {
					refs = appendNew(refs, m[1])
				}
			}
		}
	}
	return refs
}

// kitSkillDir is the kit's dir for a skill named bare or as kit:<name>, ""
// for another plugin's skill. A name the kit doesn't ship gives a dir with
// no SKILL.md, which tableReferences reads as none.
func kitSkillDir(root, name string) string {
	if plugin, skill, ok := strings.Cut(name, ":"); ok {
		if plugin != "kit" {
			return ""
		}
		name = skill
	}
	if name == "" || strings.ContainsAny(name, `/\`) {
		return ""
	}
	return filepath.Join(root, "skills", name)
}
