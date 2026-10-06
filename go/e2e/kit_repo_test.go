package e2e

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

// TestKitRepo checks the kit's own files agree with each other: what kit.yml
// names exists, skill and agent frontmatter is valid, and two skills that
// share a report shape still share it.
func TestKitRepo(t *testing.T) {
	t.Run("kit.yml holds only known keys", func(t *testing.T) {
		_, warnings, err := config.Load(config.Paths{Base: filepath.Join(kitRoot, "kit.yml"), Overlay: filepath.Join(t.TempDir(), "none.yml")})
		if err != nil || len(warnings) > 0 {
			t.Errorf("err=%v warnings=%v", err, warnings)
		}
	})

	t.Run("every skill kit.yml names exists, and every stack skill has a trigger", func(t *testing.T) {
		cfg, _, err := config.Load(config.Paths{Base: filepath.Join(kitRoot, "kit.yml"), Overlay: filepath.Join(t.TempDir(), "none.yml")})
		if err != nil {
			t.Fatal(err)
		}
		exists := func(skill string) bool {
			info, err := os.Stat(filepath.Join(kitRoot, "skills", skill))
			return err == nil && info.IsDir()
		}
		for _, rule := range cfg.SkillFileMap {
			for _, skill := range rule.Skills {
				if !exists(skill) {
					t.Errorf("skill_file_map names %q, which has no skills/ directory", skill)
				}
			}
		}
		for skill := range cfg.SkillTriggers {
			if !exists(skill) {
				t.Errorf("skill_triggers names %q, which has no skills/ directory", skill)
			}
		}
		for name, stack := range cfg.Stacks {
			skills := slices.Clone(stack.Skills)
			for _, extra := range stack.Extras {
				skills = append(skills, extra.Skills...)
			}
			for _, skill := range skills {
				if !slices.Contains(cfg.GlobalSkills, skill) && cfg.SkillTriggers[skill] == "" {
					t.Errorf("stack %s maps %q, but skill_triggers has no entry for it", name, skill)
				}
			}
		}
	})

	t.Run("skill and agent frontmatter pins only valid aliases, never model and effort together on a skill", func(t *testing.T) {
		files, _ := filepath.Glob(filepath.Join(kitRoot, "skills", "*", "SKILL.md"))
		agents, _ := filepath.Glob(filepath.Join(kitRoot, "agents", "*.md"))
		models := []string{"fable", "opus", "sonnet", "haiku", "best", "opusplan", "sonnet[1m]", "opus[1m]", "inherit", "default"}
		efforts := []string{"low", "medium", "high", "xhigh", "max"}
		// A dated id goes stale on every release; an alias always resolves.
		dated := regexp.MustCompile(`^claude-([a-z]+-)?[0-9]`)
		for _, file := range append(files, agents...) {
			rel, _ := filepath.Rel(kitRoot, file)
			front, ok := frontmatter(Read(t, file))
			if !ok {
				t.Errorf("%s: no frontmatter", rel)
				continue
			}
			model, effort := front["model"], front["effort"]
			switch {
			case model != "" && dated.MatchString(model):
				t.Errorf("%s: model %q is a dated pin; use an alias", rel, model)
			case model != "" && !slices.Contains(models, model) && !strings.HasPrefix(model, "claude-"):
				t.Errorf("%s: unknown model %q", rel, model)
			}
			if effort != "" && !slices.Contains(efforts, effort) {
				t.Errorf("%s: unknown effort %q", rel, effort)
			}
			if model == "haiku" && effort != "" {
				t.Errorf("%s: haiku takes no effort", rel)
			}
			// Claude Code drops a skill's model override when it also sets effort.
			if strings.HasPrefix(rel, "skills/") && model != "" && effort != "" {
				t.Errorf("%s: sets both model and effort; keep one", rel)
			}
		}
	})

	t.Run("audit verify parses every per-finding field the report format requires", func(t *testing.T) {
		format := Read(t, filepath.Join(kitRoot, "skills/report-format/SKILL.md"))
		verify := Read(t, filepath.Join(kitRoot, "skills/audit/reference/verify.md"))
		for _, field := range []string{"Severity", "Location", "What", "Why it matters", "Fix", "Refs"} {
			word := regexp.MustCompile(`\b` + regexp.QuoteMeta(field) + `\b`)
			if !word.MatchString(format) {
				t.Errorf("report-format no longer documents %q", field)
			}
			if !word.MatchString(verify) {
				t.Errorf("audit/reference/verify.md doesn't parse %q", field)
			}
		}
	})
}

// frontmatter is the top-level `key: value` lines between a file's opening
// pair of --- lines. Line-based, not YAML: argument-hint values like
// <a|b> aren't valid YAML scalars.
func frontmatter(text string) (map[string]string, bool) {
	body, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return nil, false
	}
	body, _, ok = strings.Cut(body, "\n---")
	if !ok {
		return nil, false
	}
	fields := map[string]string{}
	for line := range strings.SplitSeq(body, "\n") {
		if key, value, ok := strings.Cut(line, ":"); ok && !strings.HasPrefix(line, " ") {
			fields[key] = strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return fields, true
}
