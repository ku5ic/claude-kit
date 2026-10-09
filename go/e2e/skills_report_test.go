package e2e

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// kit skills-report reads $HOME/.claude/logs/skills.jsonl and the plugin
// root's kit.yml. Each test fakes $HOME to a fixture dir so real machine
// state never leaks into the assertions.
func TestSkillsReport(t *testing.T) {
	setup := func(t *testing.T) *Kit { return NewPlugin(t) }
	writeLog := func(t *testing.T, k *Kit, lines ...string) {
		Write(t, filepath.Join(k.Claude, "logs", "skills.jsonl"), strings.Join(lines, "\n")+"\n")
	}
	stamp := func(at time.Time) string { return at.UTC().Format("2006-01-02T15:04:05Z") }
	// entry is one skills.jsonl line in the full shape log-skills and
	// inject-context write; tool "" is null.
	entry := func(ts, hook, event, session, skill, tool string) string {
		var toolName any
		if tool != "" {
			toolName = tool
		}
		raw, _ := json.Marshal(map[string]any{
			"ts": ts, "hook": hook, "event": event, "session_id": session, "cwd": "/x",
			"expansion_type": nil, "command_name": nil, "command_args": nil, "command_source": nil,
			"skill_file": skill, "tool_name": toolName,
		})
		return string(raw)
	}
	skillUse := func(ts, skill string) string {
		return entry(ts, "log-skills", "PostToolUse", "s1", skill, "Skill")
	}

	// The real emitter, not a hand-written line: a field renamed in the
	// log-skills hook breaks this, where the fixture-line tests below can't.
	t.Run("a line the log-skills hook writes counts as an activation", func(t *testing.T) {
		k := setup(t)
		Mkdir(t, filepath.Join(k.Claude, "logs"))
		k.Hook("log-skills", map[string]any{
			"hook_event_name": "PostToolUse", "tool_name": "Skill", "session_id": "s1",
			"tool_input": map[string]any{"skill": "bash-patterns"},
		}).Want(t, 0)
		r := k.Run("", "skills-report")
		r.Want(t, 0)
		r.Has(t, "bash-patterns")
		r.Lacks(t, "malformed=1")
	})

	t.Run("a [days] that isn't a positive integer is an error, as in scratch-rotate", func(t *testing.T) {
		for _, days := range []string{"0", "-3", "x"} {
			r := setup(t).Run("", "skills-report", days)
			r.Want(t, 2)
			r.Has(t, "skills-report: [days] must be a positive integer, got '"+days+"'")
		}
	})

	t.Run("missing log exits 0 with a clear message", func(t *testing.T) {
		r := setup(t).Run("", "skills-report")
		r.Want(t, 0)
		r.Has(t, "no log at")
	})

	t.Run("empty log exits 0 with a clear message", func(t *testing.T) {
		k := setup(t)
		Touch(t, filepath.Join(k.Claude, "logs", "skills.jsonl"))
		r := k.Run("", "skills-report")
		r.Want(t, 0)
		r.Has(t, "is empty")
	})

	t.Run("an unreadable log fails loudly, not as an empty report", func(t *testing.T) {
		k := setup(t)
		Mkdir(t, filepath.Join(k.Claude, "logs", "skills.jsonl", "x"))
		r := k.Run("", "skills-report")
		r.Want(t, 1)
		r.Has(t, "skills-report:", "is a directory")
	})

	t.Run("malformed lines are counted and reported, not fatal", func(t *testing.T) {
		k := setup(t)
		writeLog(t, k, skillUse(stamp(time.Now()), "bash-patterns"), "not valid json{{{")
		r := k.Run("", "skills-report")
		r.Want(t, 0)
		r.Has(t, "malformed=1", "bash-patterns")
	})

	t.Run("window filter excludes entries older than the requested window", func(t *testing.T) {
		k := setup(t)
		writeLog(t, k,
			skillUse(stamp(time.Now().AddDate(0, 0, -90)), "old-skill"),
			skillUse(stamp(time.Now()), "new-skill"))

		r := k.Run("", "skills-report", "30")
		r.Has(t, "new-skill")
		r.Lacks(t, "old-skill")

		r = k.Run("", "skills-report", "120")
		r.Has(t, "old-skill", "new-skill")
	})

	t.Run("one PostToolUse Skill entry counts as one Skill-tool activation", func(t *testing.T) {
		k := setup(t)
		writeLog(t, k, skillUse(stamp(time.Now()), "bash-patterns"))
		k.Run("", "skills-report").Has(t, "1  bash-patterns")
	})

	t.Run("required-skill and suggested-skill synthetic entries are reported separately, not counted as activations", func(t *testing.T) {
		k := setup(t)
		ts := stamp(time.Now())
		writeLog(t, k,
			entry(ts, "inject-context", "required-skill", "s1", "fix-sizing", ""),
			entry(ts, "inject-context", "suggested-skill", "s2", "bash-patterns", ""))
		k.Run("", "skills-report").Has(t, "required=1", "suggested=1", "(no real activations in the window)")
	})

	t.Run("zero-activation cross-reference against a fixture kit.yml", func(t *testing.T) {
		k := setup(t)
		k.KitYML(`global_skills:
  - fix-sizing
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
  - on: basename
    globs: ["*.unused"]
    skills: [unused-patterns]
skill_triggers:
  bash-patterns: "before writing shell scripts"
stacks:
  dotfiles:
    skills: [bash-patterns]
`)
		ts := stamp(time.Now())
		writeLog(t, k,
			skillUse(ts, "bash-patterns"),
			`{"ts":"`+ts+`","hook":"log-skills","event":"UserPromptExpansion","session_id":"s1","cwd":"/x","expansion_type":"slash_command","command_name":"/flow-plan","command_args":null,"command_source":"user","skill_file":null,"tool_name":null}`)
		r := k.Run("", "skills-report")
		r.Want(t, 0)
		r.Has(t,
			"== 3: kit.yml-referenced skills with zero activations in the window ==",
			"fix-sizing",
			"unused-patterns",
			"== 4: activations for skills not referenced anywhere in kit.yml ==",
			"flow-plan")
	})

	t.Run("sessions with a suggested skill surfaced but never activated are reported", func(t *testing.T) {
		k := setup(t)
		k.KitYML(`global_skills: []
skill_file_map: []
skill_triggers:
  bash-patterns: "before writing shell scripts"
stacks:
  dotfiles:
    skills: [bash-patterns]
`)
		writeLog(t, k, entry(stamp(time.Now()), "inject-context", "suggested-skill", "s3", "bash-patterns", ""))
		k.Run("", "skills-report").Has(t, "s3: bash-patterns")
	})

	t.Run("guards section counts each rule, its disabled hits, and the last time", func(t *testing.T) {
		k := setup(t)
		ts := stamp(time.Now())
		writeLog(t, k, `{"ts":"`+ts+`","event":"PostToolUse","session_id":"s1","skill_file":"bash-patterns","tool_name":"Skill"}`)
		Write(t, filepath.Join(k.Claude, "logs", "guards.jsonl"), strings.Join([]string{
			`{"ts":"2026-01-01T00:00:00Z","hook":"guard-bash","event":"block","rule":"rm-recursive"}`,
			`{"ts":"` + ts + `","hook":"guard-bash","event":"block","rule":"find-delete"}`,
			`{"ts":"` + ts + `","hook":"guard-bash","event":"disabled","rule":"find-delete"}`,
		}, "\n")+"\n")
		r := k.Run("", "skills-report", "30")
		r.Want(t, 0)
		r.Has(t, "2  find-delete  (disabled=1 last="+ts+")")
		// Outside the 30-day window.
		r.Lacks(t, "rm-recursive")
	})
}
