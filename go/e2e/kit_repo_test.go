package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/tools"
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

	t.Run("every cited file and rules section exists", func(t *testing.T) {
		for _, bad := range brokenPointers(t) {
			t.Error(bad)
		}
	})

	t.Run("kit.yml's JS and TS extension lists cover the Go sets", func(t *testing.T) {
		cfg, _, err := config.Load(config.Paths{Base: filepath.Join(kitRoot, "kit.yml"), Overlay: filepath.Join(t.TempDir(), "none.yml")})
		if err != nil {
			t.Fatal(err)
		}
		// A list holding a language's base extension holds all of its.
		langs := map[string][]string{"js": tools.JSExtensions, "ts": tools.TSExtensions}
		covers := func(what string, exts []string) {
			for base, set := range langs {
				if !slices.Contains(exts, base) {
					continue
				}
				for _, e := range set {
					if !slices.Contains(exts, e) {
						t.Errorf("%s lists %s but not %s", what, base, e)
					}
				}
			}
		}
		for _, f := range cfg.Formatters {
			covers("formatter "+f.Name, f.Ext)
		}
		// skill_file_map: per skill, the basename globs of every rule naming it.
		bySkill := map[string][]string{}
		for _, rule := range cfg.SkillFileMap {
			for _, g := range rule.Globs {
				if ext, ok := strings.CutPrefix(g, "*."); ok && rule.On == "basename" {
					for _, skill := range rule.Skills {
						bySkill[skill] = append(bySkill[skill], ext)
					}
				}
			}
		}
		for skill, exts := range bySkill {
			covers("skill_file_map for "+skill, exts)
		}
	})

	t.Run("rules/tooling.md's skip list is kit.yml's skip_dirs", func(t *testing.T) {
		cfg, _, err := config.Load(config.Paths{Base: filepath.Join(kitRoot, "kit.yml"), Overlay: filepath.Join(t.TempDir(), "none.yml")})
		if err != nil {
			t.Fatal(err)
		}
		line := regexp.MustCompile("(?m)^- Skip for any glob, grep, or read: (.*)$").FindStringSubmatch(Read(t, filepath.Join(kitRoot, "rules/tooling.md")))
		if line == nil {
			t.Fatal("rules/tooling.md has no skip line")
		}
		var listed []string
		for _, m := range regexp.MustCompile("`([^`]+)/\\*\\*`").FindAllStringSubmatch(line[1], -1) {
			listed = append(listed, m[1])
		}
		if !slices.Equal(slices.Sorted(slices.Values(listed)), slices.Sorted(slices.Values(cfg.SkipDirs))) {
			t.Errorf("rules/tooling.md skips %q, kit.yml skip_dirs is %q", listed, cfg.SkipDirs)
		}
	})

	t.Run("the tooling rule's CLI table names every kit.yml tool", func(t *testing.T) {
		cfg, _, err := config.Load(config.Paths{Base: filepath.Join(kitRoot, "kit.yml"), Overlay: filepath.Join(t.TempDir(), "none.yml")})
		if err != nil {
			t.Fatal(err)
		}
		rule := Read(t, filepath.Join(kitRoot, "rules/tooling.md"))
		// Every word of every code span, and each word pair joined with -
		// (`git absorb` is git-absorb).
		var tools []string
		for _, m := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(rule, -1) {
			words := strings.Fields(m[1])
			for i, w := range words {
				tools = append(tools, w)
				if i+1 < len(words) {
					tools = append(tools, w+"-"+words[i+1])
				}
			}
		}
		for _, tool := range cfg.Tools {
			if !slices.Contains(tools, tool) {
				t.Errorf("kit.yml tools has %s, which rules/tooling.md section 1 doesn't name", tool)
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

var (
	// rules/x.md section 2, with or without backticks, and "sections 2 and 4".
	ruleSection = regexp.MustCompile("(rules/[a-z]+\\.md)`? sections? ([0-9]+(?:(?:, | and | or )[0-9]+)*)")
	sectionNums = regexp.MustCompile(`[0-9]+`)
	// A repo-root path, not one under ~/.claude or another directory, and not
	// a link target (linkPath reads those).
	rootPath = regexp.MustCompile(`(?:^|[^\w./~(-])((?:rules|skills|agents)/[\w./-]+\.md)\b`)
	// A markdown link target, relative to the file that holds it.
	linkPath = regexp.MustCompile(`\]\(([\w./-]+\.md)\)`)
)

// brokenPointers is every reference in the kit's prose and Go sources to a
// .md file that doesn't exist, or to a rules section with no "## N." heading.
func brokenPointers(t *testing.T) []string {
	t.Helper()
	var sources []string
	for _, dir := range []string{"rules", "agents", "skills", "go"} {
		_ = filepath.WalkDir(filepath.Join(kitRoot, dir), func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && (strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")) {
				sources = append(sources, path)
			}
			return nil
		})
	}
	sources = append(sources, filepath.Join(kitRoot, "README.md"), filepath.Join(kitRoot, "CLAUDE.md"))

	headings := map[string]bool{}
	var bad []string
	for _, src := range sources {
		rel, _ := filepath.Rel(kitRoot, src)
		for n, line := range strings.Split(Read(t, src), "\n") {
			at := func(format string, args ...any) {
				bad = append(bad, fmt.Sprintf("%s:%d: ", rel, n+1)+fmt.Sprintf(format, args...))
			}
			for _, m := range rootPath.FindAllStringSubmatch(line, -1) {
				if _, err := os.Stat(filepath.Join(kitRoot, m[1])); err != nil {
					at("%s doesn't exist", m[1])
				}
			}
			for _, m := range linkPath.FindAllStringSubmatch(line, -1) {
				if _, err := os.Stat(filepath.Join(filepath.Dir(src), m[1])); err != nil {
					at("link %s doesn't exist", m[1])
				}
			}
			for _, m := range ruleSection.FindAllStringSubmatch(line, -1) {
				for _, num := range sectionNums.FindAllString(m[2], -1) {
					key := m[1] + "#" + num
					if _, seen := headings[key]; !seen {
						text, err := os.ReadFile(filepath.Join(kitRoot, m[1]))
						headings[key] = err == nil && regexp.MustCompile(`(?m)^## `+num+`\. `).Match(text)
					}
					if !headings[key] {
						at("%s has no section %s", m[1], num)
					}
				}
			}
		}
	}
	return bad
}
