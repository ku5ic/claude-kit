package hooks

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
)

// SkillsNudge is NudgeFiles for a Read's or an edit's file_path, or a
// Grep's path and its glob joined to it. guard-dispatch runs it on an edit
// only with guard-skills off. Reads come first: in the trigger evals, a
// model that edited without the skill never went back for it once the
// edit's nudge arrived.
func SkillsNudge(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return p.Err
	}
	switch tool := p.String("tool_name"); {
	case tool == "Read" || slices.Contains(filePathTools, tool):
		return NudgeFiles(h, []string{p.FilePath()})
	case tool == "Grep":
		var paths []string
		root := searchRoot(h)
		if p.String("tool_input.path") != "" {
			paths = append(paths, root)
		}
		if glob := p.String("tool_input.glob"); glob != "" {
			paths = append(paths, filepath.Join(root, glob))
		}
		return NudgeFiles(h, paths)
	}
	return nil
}

// NudgeFiles names each patterns skill paths map to that the session hasn't
// loaded, with its "Before | Read" rows, once per skill per session, and
// lets the call through.
func NudgeFiles(h *hook.Hook, paths []string) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	if h.RuleDisabled("skills-nudge") {
		return nil
	}
	session := h.Payload.SessionID()
	marker := func(skill string) string { return h.Paths.SessionFile(cache.SkillsNudged, session, skill) }
	// Skipping a nudged skill before the skills log is read keeps every
	// later read of its file type from streaming the log again.
	nudged := func(skill string) bool { return marker(skill) == "" || fsx.IsFile(marker(skill)) }
	var b strings.Builder
	for _, skill := range unloadedSkills(h, paths, nudged) {
		if !h.DryRun {
			_ = fsx.WriteAtomic(marker(skill), nil, 0o600)
		}
		b.WriteString(skillNudge(filepath.Join(h.Paths.Root, "skills", skill), skill))
	}
	if b.Len() > 0 {
		h.AddContext("PreToolUse", strings.TrimSuffix(b.String(), "\n"))
	}
	return nil
}

// skillNudge is the nudge for one skill: its name, then one line per row of
// its SKILL.md's "Before | Read" table, the action and the file's path.
func skillNudge(dir, skill string) string {
	var b strings.Builder
	b.WriteString(skill + " covers this file. Before you edit it, load " + skill + " via the Skill tool and read the row that fits the edit:\n")
	for _, row := range tableRows(dir) {
		b.WriteString("- " + row.before + " -> " + strings.Join(row.files, ", ") + "\n")
	}
	return b.String()
}

// tableRow is one row of a "Before | Read" table: the action, then the
// absolute paths of the references it links.
type tableRow struct {
	before string
	files  []string
}

// tableLink is a reference link in a "Before | Read" table row.
var tableLink = regexp.MustCompile(`\]\((reference/[^)/]+\.md)\)`)

// tableRows is the rows of dir's SKILL.md "Before | Read" table whose
// references exist; none when it has no table.
func tableRows(dir string) []tableRow {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return nil
	}
	var rows []tableRow
	inTable := false
	for line := range strings.Lines(string(data)) {
		switch {
		case strings.HasPrefix(line, "| Before "):
			inTable = true
		case inTable && !strings.HasPrefix(line, "|"):
			inTable = false
		case inTable:
			cells := strings.Split(line, "|")
			if len(cells) < 3 {
				continue
			}
			row := tableRow{before: strings.TrimSpace(cells[1])}
			for _, m := range tableLink.FindAllStringSubmatch(line, -1) {
				if path := filepath.Join(dir, m[1]); fsx.IsFile(path) {
					row.files = append(row.files, path)
				}
			}
			if len(row.files) > 0 {
				rows = append(rows, row)
			}
		}
	}
	return rows
}
