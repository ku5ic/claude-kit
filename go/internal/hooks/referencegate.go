package hooks

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// ReferenceGate is the PreToolUse check after rules-gate on every tool: it
// denies a call until the session, or the subagent making it, has read
// every reference the "Before | Read" table of each skill it loaded links,
// naming all of them at once. A read counts until the next compaction. A
// Skill call, a hand-back, and a Read of a skill's own files pass, so the
// deny can always be met; so does an agent Claude Code gave no Read tool.
// A missing skills log fails open.
func ReferenceGate(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return p.Err
	}
	switch p.String("tool_name") {
	case "Skill", "SubagentHandback":
		return nil
	case "Read":
		if strings.Contains(p.FilePath(), "/skills/") {
			return nil
		}
	}
	agent := p.String("agent_id")
	dirs, read, err := skillReads(h.Paths.LogFile(kitlog.Skills), p.SessionID(), agent, h.Paths.Root)
	if err != nil {
		return nil
	}
	var missing []string
	for _, dir := range dirs {
		for _, ref := range tableReferences(dir) {
			if !read["skills/"+filepath.Base(dir)+"/reference/"+ref] {
				missing = append(missing, filepath.Join(dir, "reference", ref))
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	if agent != "" {
		start, _ := transcript.ReadAgentStart(subagentTranscript(p))
		if start.Tools != nil && !slices.Contains(start.Tools, "Read") {
			h.Log(kitlog.Guards, "reference-gate-unreadable", "agent_type", p.String("agent_type"))
			return nil
		}
	}
	h.Deny("read "+strings.Join(missing, ", ")+" first, then retry this call. Each is in the table of a skill you loaded, and every tool call stays blocked until they're read, so don't stop to ask", "reference-gate")
	return nil
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
				if _, err := os.Stat(filepath.Join(dir, "reference", m[1])); err == nil && !slices.Contains(refs, m[1]) {
					refs = append(refs, m[1])
				}
			}
		}
	}
	return refs
}

// skillReads streams the skills log once for what agent ("" for the main
// session) did in session: the dirs of the skills it loaded, in order, and
// the reference files it read since the last compaction, keyed from
// "skills/". A Skill call or a typed /command names a kit skill, found
// under root; a Read names its SKILL.md.
func skillReads(logPath, session, agent, root string) (dirs []string, read map[string]bool, err error) {
	if _, err := os.Stat(logPath); err != nil {
		return nil, nil, err
	}
	read = map[string]bool{}
	addDir := func(dir string) {
		if dir != "" && !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
	}
	quoted := []byte(`"` + session + `"`)
	err = kitlog.Each(logPath, func(line []byte) {
		// Most lines are other sessions'; skip them before decoding.
		if !bytes.Contains(line, quoted) {
			return
		}
		var entry kitlog.Entry
		if json.Unmarshal(line, &entry) != nil || entry.SessionID == nil || *entry.SessionID != session {
			return
		}
		if entry.Event == kitlog.EventCompacted {
			clear(read)
			return
		}
		if entry.AgentID != agent || entry.Surfaced() {
			return
		}
		switch {
		case entry.SkillFile != nil && strings.HasSuffix(*entry.SkillFile, "/SKILL.md"):
			addDir(filepath.Dir(*entry.SkillFile))
		case entry.SkillFile != nil && strings.Contains(*entry.SkillFile, "/reference/"):
			if i := strings.LastIndex(*entry.SkillFile, "/skills/"); i >= 0 {
				read[(*entry.SkillFile)[i+1:]] = true
			}
		case entry.SkillFile != nil:
			addDir(kitSkillDir(root, *entry.SkillFile))
		case entry.CommandName != nil:
			addDir(kitSkillDir(root, strings.TrimPrefix(*entry.CommandName, "/")))
		}
	})
	return dirs, read, err
}

// kitSkillDir is the kit's dir for a skill named bare or as kit:<name>, ""
// for another plugin's skill or one the kit doesn't ship.
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
	dir := filepath.Join(root, "skills", name)
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		return ""
	}
	return dir
}
