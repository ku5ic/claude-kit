// Package report is skills-report: skill activation telemetry from
// skills.jsonl and guard rule firings from guards.jsonl. Read-only: it never
// writes, trims, or rotates a log. Malformed lines are counted, never fatal.
package report

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

// entry is one log row; Skill is a pointer so a null stays distinct from "".
type entry struct {
	TS            *string `json:"ts"`
	Event         string  `json:"event"`
	SessionID     *string `json:"session_id"`
	ExpansionType string  `json:"expansion_type"`
	CommandName   *string `json:"command_name"`
	SkillFile     *string `json:"skill_file"`
	ToolName      string  `json:"tool_name"`
	Rule          *string `json:"rule"`

	category string
	skill    *string
}

func (e entry) ts() string { return deref(e.TS) }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// jqText renders a nullable string as jq's "\(.x)" does.
func jqText(s *string) string {
	if s == nil {
		return "null"
	}
	return *s
}

// readLog parses a JSONL file: blank lines are dropped, unparsable or
// non-object lines counted as malformed.
func readLog(path string) (entries []entry, malformed int) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for s.Scan() {
		line := s.Text()
		if line == "" {
			continue
		}
		var e entry
		if strings.HasPrefix(strings.TrimSpace(line), "{") && json.Unmarshal([]byte(line), &e) == nil {
			entries = append(entries, e)
		} else {
			malformed++
		}
	}
	return entries, malformed
}

var readSkill = regexp.MustCompile(`/skills/([^/]+)/SKILL\.md$`)

// classify sets category and skill; ok is false for an entry the report
// ignores.
func classify(e *entry) bool {
	switch {
	case e.Event == "UserPromptExpansion" && e.ExpansionType == "slash_command":
		e.category = "slash_command"
		name := strings.TrimPrefix(deref(e.CommandName), "/")
		if i := strings.IndexByte(name, ':'); i >= 0 {
			name = name[:i]
		}
		e.skill = &name
	case e.Event == "PostToolUse" && e.ToolName == "Skill":
		e.category, e.skill = "skill_tool", e.SkillFile
	case e.Event == "PostToolUse" && e.ToolName == "Read":
		e.category = "read_fallback"
		if e.SkillFile == nil {
			e.skill = nil
			return true
		}
		m := readSkill.FindStringSubmatch(*e.SkillFile)
		if m == nil {
			// jq's capture yields nothing on no match, dropping the row.
			return false
		}
		e.skill = &m[1]
	case e.Event == "required-skill":
		e.category, e.skill = "surfaced_required", e.SkillFile
	case e.Event == "suggested-skill":
		e.category, e.skill = "surfaced_suggested", e.SkillFile
	default:
		return false
	}
	return true
}

func real(category string) bool {
	return category == "slash_command" || category == "skill_tool" || category == "read_fallback"
}

// group is jq's group_by(.skill) key order: null first, then strings.
type group struct {
	key  *string
	rows []entry
}

func groupBy(rows []entry, key func(entry) *string) []group {
	var groups []group
	for _, r := range rows {
		k := key(r)
		i := slices.IndexFunc(groups, func(g group) bool {
			return (g.key == nil && k == nil) || (g.key != nil && k != nil && *g.key == *k)
		})
		if i < 0 {
			groups = append(groups, group{key: k})
			i = len(groups) - 1
		}
		groups[i].rows = append(groups[i].rows, r)
	}
	sort.SliceStable(groups, func(a, b int) bool {
		ka, kb := groups[a].key, groups[b].key
		if ka == nil || kb == nil {
			return ka == nil && kb != nil
		}
		return *ka < *kb
	})
	return groups
}

func count(rows []entry, category string) int {
	n := 0
	for _, r := range rows {
		if r.category == category {
			n++
		}
	}
	return n
}

// Run is skills-report.sh [days].
func Run(cfg *config.Config, paths config.Paths, args []string, stdout io.Writer) int {
	days := 30
	if len(args) > 0 {
		if n, err := strconv.Atoi(args[0]); err == nil {
			days = n
		}
	}
	logFile := filepath.Join(paths.LogDir(), "skills.jsonl")
	info, err := os.Stat(logFile)
	if err != nil {
		fmt.Fprintf(stdout, "skills-report: no log at %s, nothing to report\n", logFile)
		return 0
	}
	if info.Size() == 0 {
		fmt.Fprintf(stdout, "skills-report: %s is empty, nothing to report\n", logFile)
		return 0
	}
	all, malformed := readLog(logFile)
	if len(all) == 0 {
		fmt.Fprintf(stdout, "skills-report: no valid JSONL lines in %s (%d malformed)\n", logFile, malformed)
		return 0
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02T15:04:05Z")
	var entries []entry
	for _, e := range all {
		if e.TS != nil && e.ts() >= cutoff {
			entries = append(entries, e)
		}
	}
	fmt.Fprintf(stdout, "skills-report: window=%dd cutoff=%s entries=%d malformed=%d\n\n", days, cutoff, len(entries), malformed)
	if len(entries) == 0 {
		fmt.Fprintf(stdout, "no entries in the last %d day(s)\n", days)
		return 0
	}

	var rows, active []entry
	for _, e := range entries {
		if classify(&e) {
			rows = append(rows, e)
			if real(e.category) {
				active = append(active, e)
			}
		}
	}
	skillOf := func(e entry) *string { return e.skill }

	fmt.Fprintln(stdout, "== 1+2: activation counts per skill, by path (real activations only) ==")
	groups := groupBy(active, skillOf)
	sort.SliceStable(groups, func(a, b int) bool { return len(groups[a].rows) > len(groups[b].rows) })
	if len(groups) == 0 {
		fmt.Fprintln(stdout, "(no real activations in the window)")
	}
	for _, g := range groups {
		fmt.Fprintf(stdout, "%d  %s  (slash=%d skill_tool=%d read=%d)\n", len(g.rows), jqText(g.key),
			count(g.rows, "slash_command"), count(g.rows, "skill_tool"), count(g.rows, "read_fallback"))
	}

	fmt.Fprintln(stdout, "\n== 2b: surfaced only, not confirmed loaded (required-skill / suggested-skill markers) ==")
	var surfacedRows []entry
	for _, r := range rows {
		if r.category == "surfaced_required" || r.category == "surfaced_suggested" {
			surfacedRows = append(surfacedRows, r)
		}
	}
	surfaced := groupBy(surfacedRows, skillOf)
	sort.SliceStable(surfaced, func(a, b int) bool { return len(surfaced[a].rows) > len(surfaced[b].rows) })
	if len(surfaced) == 0 {
		fmt.Fprintln(stdout, "(none in the window)")
	}
	for _, g := range surfaced {
		fmt.Fprintf(stdout, "%s  required=%d suggested=%d\n", jqText(g.key), count(g.rows, "surfaced_required"), count(g.rows, "surfaced_suggested"))
	}

	if cfg == nil {
		fmt.Fprintln(stdout, "\n== 3+4: skipped (kit.yml not available) ==")
	} else {
		referencedSections(cfg, groups, stdout)
	}
	suggestedSection(rows, active, stdout)
	guardsSection(paths, cutoff, stdout)
	return 0
}

func referencedSections(cfg *config.Config, groups []group, stdout io.Writer) {
	referenced := referencedSkills(cfg)
	var activeNames []string
	for _, g := range groups {
		activeNames = append(activeNames, jqText(g.key))
	}
	fmt.Fprintln(stdout, "\n== 3: kit.yml-referenced skills with zero activations in the window ==")
	zero := minus(referenced, activeNames)
	if len(zero) == 0 {
		fmt.Fprintln(stdout, "(none - every referenced skill was activated at least once in the window)")
	}
	for _, s := range zero {
		fmt.Fprintln(stdout, s)
	}
	fmt.Fprintln(stdout, "\n== 4: activations for skills not referenced anywhere in kit.yml ==")
	fmt.Fprintln(stdout, "(expected for audit, write, meta, deps, and investigate procedure skills -- kit.yml only maps pattern/reference skills to stacks, not this group)")
	unreferenced := minus(activeNames, referenced)
	if len(unreferenced) == 0 {
		fmt.Fprintln(stdout, "(none)")
	}
	for _, s := range unreferenced {
		fmt.Fprintln(stdout, s)
	}
}

func suggestedSection(rows, active []entry, stdout io.Writer) {
	fmt.Fprintln(stdout, "\n== 5: sessions with a suggested skill surfaced but never activated in that session ==")
	activated := map[string]bool{}
	for _, e := range active {
		activated[jqText(e.SessionID)+"::"+jqText(e.skill)] = true
	}
	var suggested []entry
	for _, r := range rows {
		if r.category == "surfaced_suggested" {
			suggested = append(suggested, r)
		}
	}
	any5 := false
	for _, g := range groupBy(suggested, func(e entry) *string { return e.SessionID }) {
		var missed []string
		for _, e := range g.rows {
			if name := jqText(e.skill); !activated[jqText(e.SessionID)+"::"+name] && !slices.Contains(missed, name) {
				missed = append(missed, name)
			}
		}
		if len(missed) > 0 {
			slices.Sort(missed)
			fmt.Fprintf(stdout, "%s: %s\n", jqText(g.key), strings.Join(missed, ", "))
			any5 = true
		}
	}
	if !any5 {
		fmt.Fprintln(stdout, "(none - either no suggested-skill markers in the window, or every one was followed)")
	}
	fmt.Fprintln(stdout, "(sessions before suggested-skill logging landed have no marker and are excluded above, not counted as followed)")
}

func guardsSection(paths config.Paths, cutoff string, stdout io.Writer) {
	fmt.Fprintln(stdout, "\n== 6: guard rules fired in the window (guards.jsonl) ==")
	guards, _ := readLog(filepath.Join(paths.LogDir(), "guards.jsonl"))
	var recent []entry
	for _, e := range guards {
		if e.TS != nil && e.ts() >= cutoff {
			recent = append(recent, e)
		}
	}
	rules := groupBy(recent, func(e entry) *string { return e.Rule })
	sort.SliceStable(rules, func(a, b int) bool { return len(rules[a].rows) > len(rules[b].rows) })
	if len(rules) == 0 {
		fmt.Fprintln(stdout, "(none in the window)")
	}
	for _, g := range rules {
		name := "(no slug)"
		if g.key != nil {
			name = *g.key
		}
		disabled, last := 0, ""
		for _, e := range g.rows {
			if e.Event == "disabled" {
				disabled++
			}
			last = max(last, e.ts())
		}
		fmt.Fprintf(stdout, "%d  %s  (disabled=%d last=%s)\n", len(g.rows), name, disabled, last)
	}
}

// referencedSkills is every skill kit.yml names: global_skills, stack and
// extra skills, and skill_file_map, unique and sorted.
func referencedSkills(cfg *config.Config) []string {
	var out []string
	add := func(skills []string) {
		for _, s := range skills {
			if s != "" && !slices.Contains(out, s) {
				out = append(out, s)
			}
		}
	}
	add(cfg.GlobalSkills)
	for _, name := range cfg.StackOrder {
		add(cfg.Stacks[name].Skills)
		for _, e := range cfg.Stacks[name].Extras {
			add(e.Skills)
		}
	}
	for _, rule := range cfg.SkillFileMap {
		add(rule.Skills)
	}
	slices.Sort(out)
	return out
}

// minus is jq's array difference, sorted.
func minus(a, b []string) []string {
	var out []string
	for _, s := range a {
		if !slices.Contains(b, s) {
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out
}
