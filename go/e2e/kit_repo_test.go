package e2e

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/codegraph"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/slop"
	"github.com/ku5ic/claude-kit/go/internal/stackctx"
	"github.com/ku5ic/claude-kit/go/internal/weaken"
)

// TestKitRepo checks the kit's own files agree with each other: what kit.yml
// names exists, skill and agent frontmatter is valid, and two skills that
// share a report shape still share it.
func TestKitRepo(t *testing.T) {
	t.Parallel()
	t.Run("kit.yml holds only known keys", func(t *testing.T) {
		_, warnings, err := config.Load(config.Paths{Base: filepath.Join(kitRoot, "kit.yml"), Overlay: filepath.Join(t.TempDir(), "none.yml")})
		if err != nil || len(warnings) > 0 {
			t.Errorf("err=%v warnings=%v", err, warnings)
		}
	})

	t.Run("every /config choice in plugin.json is a value the kit accepts, and every row is titled Kit:", func(t *testing.T) {
		var manifest struct {
			UserConfig map[string]struct {
				Title   string   `json:"title"`
				Options []string `json:"options"`
				Default any      `json:"default"`
			} `json:"userConfig"`
		}
		data, err := os.ReadFile(filepath.Join(kitRoot, ".claude-plugin", "plugin.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatal(err)
		}
		presets, err := filepath.Glob(filepath.Join(kitRoot, "presets", "*.yml"))
		if err != nil {
			t.Fatal(err)
		}
		var presetNames []string
		for _, path := range presets {
			presetNames = append(presetNames, strings.TrimSuffix(filepath.Base(path), ".yml"))
		}
		cfg, _, err := config.Load(config.Paths{Base: filepath.Join(kitRoot, "kit.yml")})
		if err != nil {
			t.Fatal(err)
		}
		pickers := map[string][]string{
			"preset":       presetNames,
			"step_pause":   config.StepPauses,
			"receipt":      config.Receipts,
			"reply_length": slices.Collect(maps.Keys(cfg.ReplyLimits.Modes)),
			"reply_slack":  slices.Collect(maps.Keys(config.ReplySlacks)),
		}
		for key, accepted := range pickers {
			option, ok := manifest.UserConfig[key]
			if !ok {
				t.Errorf("userConfig has no %s row", key)
				continue
			}
			if option.Default != "default" || !slices.Contains(option.Options, "default") {
				t.Errorf("%s: default %v, options %v, want a default choice that's also the default", key, option.Default, option.Options)
			}
			for _, choice := range option.Options {
				if choice != "default" && !slices.Contains(accepted, choice) {
					t.Errorf("%s offers %q, which the kit doesn't accept (%v)", key, choice, accepted)
				}
			}
		}
		for key, option := range manifest.UserConfig {
			if !strings.HasPrefix(option.Title, "Kit: ") {
				t.Errorf("%s: title %q doesn't start with \"Kit: \"", key, option.Title)
			}
		}
	})

	t.Run("Java test-file globs name tests, not production files that start with Test", func(t *testing.T) {
		cfg, _, err := config.Load(config.Paths{Base: filepath.Join(kitRoot, "kit.yml"), Overlay: filepath.Join(t.TempDir(), "none.yml")})
		if err != nil {
			t.Fatal(err)
		}
		for path, test := range map[string]bool{
			"src/test/java/com/acme/TestStrings.java":   true,
			"src/main/java/com/acme/Testimonial.java":   false,
			"src/main/java/com/acme/Testing/Util.java":  false,
			"src/test/java/com/acme/StringsTest.java":   true,
			"src/main/java/com/acme/TestimonialBo.java": false,
		} {
			if weaken.IsCode(cfg.WeakenedChecks, path) == test {
				t.Errorf("weakened_checks: %s is a test file = %v, want %v", path, !test, test)
			}
			if slices.Contains(stackctx.FileMapSkills(cfg.SkillFileMap, path), "test-patterns") != test {
				t.Errorf("skill_file_map: %s loads test-patterns = %v, want %v", path, !test, test)
			}
		}
	})

	t.Run("every skill kit.yml names exists, and every suggested skill has a trigger", func(t *testing.T) {
		cfg, _, err := config.Load(config.Paths{Base: filepath.Join(kitRoot, "kit.yml"), Overlay: filepath.Join(t.TempDir(), "none.yml")})
		if err != nil {
			t.Fatal(err)
		}
		exists := func(skill string) bool {
			info, err := os.Stat(filepath.Join(kitRoot, "skills", skill))
			return err == nil && info.IsDir()
		}
		mapped := map[string][]string{}
		for _, rule := range cfg.SkillFileMap {
			mapped["skill_file_map"] = append(mapped["skill_file_map"], rule.Skills...)
		}
		for _, rule := range cfg.DependencySkills {
			mapped["dependency_skills"] = append(mapped["dependency_skills"], rule.Skills...)
		}
		for key, skills := range mapped {
			for _, skill := range skills {
				if !exists(skill) {
					t.Errorf("%s names %q, which has no skills/ directory", key, skill)
				}
				if !slices.Contains(cfg.GlobalSkills, skill) && cfg.SkillTriggers[skill] == "" {
					t.Errorf("%s suggests %q, but skill_triggers has no entry for it", key, skill)
				}
			}
		}
		for skill := range cfg.SkillTriggers {
			if !exists(skill) {
				t.Errorf("skill_triggers names %q, which has no skills/ directory", skill)
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

	t.Run("every agent caps its turns and has a color Claude Code draws", func(t *testing.T) {
		agents, _ := filepath.Glob(filepath.Join(kitRoot, "agents", "*.md"))
		if len(agents) == 0 {
			t.Fatal("no agents found")
		}
		// code.claude.com/docs/en/sub-agents, Supported frontmatter fields.
		colors := []string{"red", "blue", "green", "yellow", "purple", "orange", "pink", "cyan"}
		for _, file := range agents {
			rel, _ := filepath.Rel(kitRoot, file)
			front, _ := frontmatter(Read(t, file))
			if n, err := strconv.Atoi(front["maxTurns"]); err != nil || n < 1 {
				t.Errorf("%s: maxTurns %q isn't a number of turns", rel, front["maxTurns"])
			}
			if !slices.Contains(colors, front["color"]) {
				t.Errorf("%s: color %q isn't one Claude Code documents", rel, front["color"])
			}
		}
	})

	// Set from the sizes after sources moved out; step 18's rewrite lowers them.
	const maxDescription, maxSkill = 200, 5 * 1024
	t.Run("a reference skill keeps its sources in sources.md and fits its budget", func(t *testing.T) {
		// The skills that aren't reference skills, so keep no sources.md;
		// skills/meta-refresh/SKILL.md points here for the list.
		// A skill only the user invokes (disable-model-invocation) is a
		// procedure skill. It keeps its sources too (skills/meta-skill/SKILL.md,
		// Rules), but its description and SKILL.md load only when the user
		// types it, so no budget holds them.
		exempt := []string{"investigate", "report-format", "kit-rules", "slop-patterns"}
		// Reference skills with no version-dependent content: no sources.md,
		// and /meta-refresh takes them only when named.
		unversioned := []string{
			"engineering-fundamentals",
			// Procedure skills whose practice another skill owns: the stack
			// pattern skills, wcag-audit, slop-patterns, code-host-patterns,
			// engineering-fundamentals' thresholds, or meta-skill's and
			// meta-prompt's criteria.
			"audit-a11y", "audit-debt", "audit-infra", "audit-skills", "audit-slop", "audit-tests", "pr-publish", "unslop",
			// Procedure skills that run only the kit's own steps, with no
			// outside practice to cite.
			"audit-doc-drift", "audit-verify", "code-impact", "code-tour", "meta-refresh",
			"write-devnote", "write-explainer",
		}
		url := regexp.MustCompile(`https?://`)
		dirs, _ := filepath.Glob(filepath.Join(kitRoot, "skills", "*", "SKILL.md"))
		for _, skill := range dirs {
			dir := filepath.Dir(skill)
			text := Read(t, skill)
			front, _ := frontmatter(text)
			procedure := front["disable-model-invocation"] == "true"
			refs, _ := filepath.Glob(filepath.Join(dir, "reference", "*.md"))
			if slices.Contains(exempt, filepath.Base(dir)) {
				continue
			}
			rel, _ := filepath.Rel(kitRoot, skill)
			// /meta-refresh reads it for every skill outside exempt.
			if _, err := os.Stat(filepath.Join(dir, "sources.md")); err != nil && !slices.Contains(unversioned, filepath.Base(dir)) {
				t.Errorf("%s: no sources.md; add it, or the skill to exempt if it's not a reference skill", rel)
			}
			if n := len(front["description"]); !procedure && (n == 0 || n > maxDescription) {
				t.Errorf("%s: description is %d characters, want 1 to %d", rel, n, maxDescription)
			}
			if !procedure && len(text) > maxSkill {
				t.Errorf("%s: %d bytes, over the %d-byte budget", rel, len(text), maxSkill)
			}
			// The two skills that write and refresh sources.md name it.
			if strings.Contains(text, "sources.md") && !slices.Contains([]string{"meta-skill", "meta-refresh"}, filepath.Base(dir)) {
				t.Errorf("%s: names sources.md, which a session must never load", rel)
			}
			for _, file := range append(refs, skill) {
				rel, _ := filepath.Rel(kitRoot, file)
				for n, line := range proseLines(Read(t, file)) {
					if line == "## References" || line == "## Version notes" || url.MatchString(line) {
						t.Errorf("%s:%d: a source belongs in sources.md: %s", rel, n, line)
					}
				}
			}
		}
	})

	t.Run("kit-rules' SKILL.md, whose hard rules a compaction carries over, fits the same budget", func(t *testing.T) {
		if n := len(Read(t, filepath.Join(kitRoot, "skills", "kit-rules", "SKILL.md"))); n > maxSkill {
			t.Errorf("skills/kit-rules/SKILL.md: %d bytes, over the %d-byte budget", n, maxSkill)
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
		langs := map[string][]string{"js": codegraph.JSExtensions, "ts": codegraph.TSExtensions}
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

	// Where a language lives in the kit: every language code-graph reads
	// imports of, the slop guard reads comments of and skill_file_map
	// loads engineering-fundamentals for. /add-language follows this list.
	t.Run("every language code-graph scans is read by slop and skill_file_map", func(t *testing.T) {
		cfg, _, err := config.Load(config.Paths{Base: filepath.Join(kitRoot, "kit.yml"), Overlay: filepath.Join(t.TempDir(), "none.yml")})
		if err != nil {
			t.Fatal(err)
		}
		for _, ext := range codegraph.Extensions() {
			file := "a" + ext
			if !slop.Reads(file) {
				t.Errorf("slop has no comment syntax for %s (go/internal/slop/segments.go)", ext)
			}
			if !slices.Contains(stackctx.FileMapSkills(cfg.SkillFileMap, file), "engineering-fundamentals") {
				t.Errorf("skill_file_map loads engineering-fundamentals for no %s file (kit.yml)", ext)
			}
		}
	})

	t.Run("a skill with a Before | Read table links every reference it has", func(t *testing.T) {
		skills, err := filepath.Glob(filepath.Join(kitRoot, "skills", "*", "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		gated := 0
		for _, skillMD := range skills {
			text := Read(t, skillMD)
			if !strings.Contains(text, "| Before ") {
				continue
			}
			gated++
			refs, _ := filepath.Glob(filepath.Join(filepath.Dir(skillMD), "reference", "*.md"))
			for _, ref := range refs {
				if !strings.Contains(text, "](reference/"+filepath.Base(ref)+")") {
					t.Errorf("%s doesn't link reference/%s, so no row ever sends Claude to it", skillMD, filepath.Base(ref))
				}
			}
		}
		if gated == 0 {
			t.Fatal("no skill has a Before | Read table")
		}
	})

	t.Run("section 4's table gives each kind of task a model and effort the Agent call takes", func(t *testing.T) {
		const rule = "skills/kit-rules/reference/subagents.md"
		text := Read(t, filepath.Join(kitRoot, rule))
		tasks := tableColumn(text, "## 4. Fan-out", "Task")
		models := tableColumn(text, "## 4. Fan-out", "Model")
		efforts := tableColumn(text, "## 4. Fan-out", "Effort")
		if !slices.Equal(tasks, []string{"Collect", "Check", "Judge"}) || len(models) != 3 || len(efforts) != 3 {
			t.Fatalf("%s section 4: tasks %v, models %v, efforts %v", rule, tasks, models, efforts)
		}
		for i, task := range tasks {
			// The aliases the Agent tool's model parameter takes.
			if !slices.Contains([]string{"haiku", "sonnet", "opus", "fable"}, models[i]) {
				t.Errorf("%s: model %q isn't one the Agent call takes", task, models[i])
			}
			unset := efforts[i] == "unset" && models[i] == "haiku"
			if !unset && !slices.Contains([]string{"low", "medium", "high", "xhigh", "max"}, efforts[i]) {
				t.Errorf("%s: effort %q isn't a level, or unset with haiku", task, efforts[i])
			}
		}
	})

	t.Run("docs/dependencies.md has a row for every kit.yml tool", func(t *testing.T) {
		cfg, _, err := config.Load(config.Paths{Base: filepath.Join(kitRoot, "kit.yml"), Overlay: filepath.Join(t.TempDir(), "none.yml")})
		if err != nil {
			t.Fatal(err)
		}
		page := Read(t, filepath.Join(kitRoot, "docs", "dependencies.md"))
		for _, tool := range cfg.Tools {
			if !regexp.MustCompile("(?m)^\\| `" + regexp.QuoteMeta(tool) + "` +\\|").MatchString(page) {
				t.Errorf("kit.yml tools has %s, which docs/dependencies.md has no row for", tool)
			}
		}
	})

	t.Run("docs/commands.md lists every command and docs/skills.md every other skill", func(t *testing.T) {
		commands := Read(t, filepath.Join(kitRoot, "docs", "commands.md"))
		skills := Read(t, filepath.Join(kitRoot, "docs", "skills.md"))
		dirs, _ := filepath.Glob(filepath.Join(kitRoot, "skills", "*", "SKILL.md"))
		for _, skill := range dirs {
			name := filepath.Base(filepath.Dir(skill))
			front, _ := frontmatter(Read(t, skill))
			page, row, rel := skills, "`"+name+"`", "docs/skills.md"
			if front["disable-model-invocation"] == "true" {
				page, row, rel = commands, "`/"+name+"`", "docs/commands.md"
			}
			if !regexp.MustCompile("(?m)^\\| " + regexp.QuoteMeta(row) + " +\\|").MatchString(page) {
				t.Errorf("%s has no row for %s", rel, row)
			}
		}
	})

	t.Run("the tooling rule's CLI table names every kit.yml tool", func(t *testing.T) {
		cfg, _, err := config.Load(config.Paths{Base: filepath.Join(kitRoot, "kit.yml"), Overlay: filepath.Join(t.TempDir(), "none.yml")})
		if err != nil {
			t.Fatal(err)
		}
		rule := Read(t, filepath.Join(kitRoot, "skills/kit-rules/reference/tooling.md"))
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
				t.Errorf("kit.yml tools has %s, which skills/kit-rules/reference/tooling.md section 1 doesn't name", tool)
			}
		}
	})

	// Each file names a finding's parts in one sentence: report-format's
	// "A finding's parts:" line, and verify's step 3 "Extract every
	// finding's" line. The two group the parts differently, so each line
	// only has to hold every field, in any order.
	t.Run("audit verify parses every per-finding field the report format requires", func(t *testing.T) {
		for file, opening := range map[string]string{
			"skills/report-format/SKILL.md": "A finding's parts:",
			"skills/audit-verify/SKILL.md":  "Extract every finding's",
		} {
			var parts string
			for line := range strings.SplitSeq(Read(t, filepath.Join(kitRoot, file)), "\n") {
				if strings.Contains(line, opening) {
					parts = line
				}
			}
			if parts == "" {
				t.Errorf("%s has no line with %q", file, opening)
				continue
			}
			for _, field := range []string{"severity", "problem", "location", "Evidence", "Why it matters", "Fix", "Refs"} {
				if !regexp.MustCompile(`\b` + regexp.QuoteMeta(field) + `\b`).MatchString(parts) {
					t.Errorf("%s: its %q line lacks %q", file, opening, field)
				}
			}
		}
	})
}

// tableColumn is the cells under column in the first table of the section
// heading opens, without backticks.
func tableColumn(text, heading, column string) []string {
	_, section, ok := strings.Cut(text, "\n"+heading+"\n")
	if !ok {
		return nil
	}
	section, _, _ = strings.Cut(section, "\n## ")
	at := -1
	var cells []string
	for line := range strings.SplitSeq(section, "\n") {
		row, isRow := strings.CutPrefix(line, "|")
		switch {
		case !isRow && at >= 0:
			return cells
		case !isRow:
			continue
		}
		cols := strings.Split(row, "|")
		for i := range cols {
			cols[i] = strings.Trim(strings.TrimSpace(cols[i]), "`")
		}
		switch {
		case at < 0:
			at = slices.Index(cols, column)
		case at < len(cols) && !strings.HasPrefix(cols[at], "--"):
			cells = append(cells, cols[at])
		}
	}
	return cells
}

// proseLines is text's lines by 1-based number, without fenced code and
// with inline code spans blanked, so an example URL in code doesn't count.
func proseLines(text string) map[int]string {
	span := regexp.MustCompile("`[^`]*`")
	lines := map[int]string{}
	inFence := false
	for i, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if !inFence {
			lines[i+1] = span.ReplaceAllString(line, "``")
		}
	}
	return lines
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
	// A rule file's section 2, with or without backticks, and "sections 2 and 4".
	ruleSection = regexp.MustCompile("(skills/kit-rules/reference/[a-z]+\\.md)`? sections? ([0-9]+(?:(?:, | and | or )[0-9]+)*)")
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
	for _, dir := range []string{"agents", "skills", "go", "docs", ".claude/skills"} {
		if _, err := os.Stat(filepath.Join(kitRoot, dir)); err != nil {
			t.Error(err)
		}
		_ = filepath.WalkDir(filepath.Join(kitRoot, dir), func(path string, d os.DirEntry, err error) error {
			// An accepted ADR changes only in its status line, so a pointer
			// in one that a later move breaks can't be fixed.
			if err == nil && d.IsDir() && d.Name() == "adr" {
				return filepath.SkipDir
			}
			if err == nil && !d.IsDir() && (strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")) {
				sources = append(sources, path)
			}
			return nil
		})
	}
	sources = append(sources, filepath.Join(kitRoot, "README.md"), filepath.Join(kitRoot, "CLAUDE.md"), filepath.Join(kitRoot, "CONTRIBUTING.md"))

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
