package chatbundle

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestChatTextMatchesTheGoldens(t *testing.T) {
	t.Parallel()
	rules := map[string]rule{
		"rules/change.md":   {"Change", []string{"5", "8", "10"}},
		"rules/evidence.md": {"Evidence", []string{"1"}},
		"rules/output.md":   {"Output", []string{"1", "4"}},
	}
	b := &bundle{files: map[string][]byte{"skills/write/reference/commit.md": nil}}
	for _, name := range []string{"commit", "chat", "SKILL"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in := testutil.Read(t, filepath.Join("testdata", name+".md"))
			got, errs := b.chatText(name+".md", in, rules)
			if len(errs) > 0 {
				t.Fatal(errs)
			}
			if want := testutil.Read(t, filepath.Join("testdata", name+".golden")); got != want {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestMarkers(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ in, want string }{
		"no markers is untouched": {"a\n\n\nb\n", "a\n\n\nb\n"},
		"a skip leaves one blank": {"a\n\n<!-- chat: skip -->\n\nb\n\n<!-- chat: end -->\n\nc\n", "a\n\nc\n"},
		"only keeps its text":     {"a\n\n<!-- chat: only\nb\n-->\n\nc\n", "a\n\nb\n\nc\n"},
		"indented in a list":      {"- a\n  <!-- chat: skip -->\n  b\n  <!-- chat: end -->\n- c\n", "- a\n- c\n"},
		"other comments stay":     {"<!-- failure -->\n<!-- unslop: allow -->\n", "<!-- failure -->\n<!-- unslop: allow -->\n"},
		"a fence's marker stays":  {"```\n<!-- chat: skip -->\n```\n", "```\n<!-- chat: skip -->\n```\n"},
		"a skip swallows a fence": {"<!-- chat: skip -->\n```\n<!-- chat: end -->\n```\n<!-- chat: end -->\nb\n", "b\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := applyMarkers(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBadMarkers(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ in, want string }{
		"unclosed skip":    {"a\n<!-- chat: skip -->\nb\n", "line 2: marker never closed"},
		"unclosed only":    {"<!-- chat: only\nb\n", "line 1: marker never closed"},
		"stray end":        {"<!-- chat: end -->\n", "bad marker"},
		"misspelled":       {"<!-- chat: skp -->\n", "bad marker"},
		"one-line only":    {"<!-- chat: only b -->\n", "bad marker"},
		"nested":           {"<!-- chat: skip -->\n<!-- chat: only\n", "line 2: marker inside the one opened at line 1"},
		"end in an only":   {"<!-- chat: only\n<!-- chat: end -->\n-->\n", "marker inside"},
		"only in a skip":   {"<!-- chat: only\n<!-- chat: skip -->\n-->\n", "marker inside"},
		"skip in a skip":   {"<!-- chat: skip -->\n<!-- chat: skip -->\n<!-- chat: end -->\n", "marker inside"},
		"close is no skip": {"<!-- chat: skip -->\n-->\n", "never closed"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := applyMarkers(tc.in); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err %v, want %q", err, tc.want)
			}
		})
	}
}

func TestParseRule(t *testing.T) {
	t.Parallel()
	got, err := parseRule("# Change\n\n## 1. One\n\n```\n## 2. In a fence\n```\n\n## 10. Ten\n")
	if err != nil {
		t.Fatal(err)
	}
	if got.title != "Change" || strings.Join(got.sections, " ") != "1 10" {
		t.Errorf("got %+v", got)
	}
	if _, err := parseRule("no title\n"); err == nil {
		t.Error("a rule with no title passed")
	}
}

func TestLinks(t *testing.T) {
	t.Parallel()
	rules := map[string]rule{"rules/change.md": {"Change", []string{"1", "2", "3"}}}
	b := &bundle{files: map[string][]byte{"skills/a/reference/x.md": nil}}
	for name, tc := range map[string]struct{ in, want string }{
		"bare rule":        {"`rules/change.md`", "the kit-rules skill, Change"},
		"possessive":       {"`rules/change.md`'s rule", "the kit-rules skill, Change's rule"},
		"one section":      {"`rules/change.md` section 2.", "the kit-rules skill, Change section 2."},
		"several sections": {"`rules/change.md` sections 1, 2, and 3", "the kit-rules skill, Change sections 1, 2, and 3"},
		"two sections":     {"`rules/change.md` sections 1 and 2", "the kit-rules skill, Change sections 1 and 2"},
		"unbundled rule":   {"`rules/verify.md` section 9", "`rules/verify.md` section 9"},
		"bundled skill":    {"`skills/a/reference/x.md`", "`reference/x.md` in the a skill"},
		"unbundled skill":  {"`skills/b/reference/x.md`", "`skills/b/reference/x.md`"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, errs := b.rewriteLinks("f.md", tc.in, rules)
			if len(errs) > 0 {
				t.Fatal(errs)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLinkToADroppedSectionFails(t *testing.T) {
	t.Parallel()
	rules := map[string]rule{"rules/change.md": {"Change", []string{"1", "3"}}}
	_, errs := (&bundle{}).rewriteLinks("f.md", "`rules/change.md` sections 1, 2, and 4", rules)
	if len(errs) != 2 || !strings.Contains(errs[0].Error(), "f.md: links rules/change.md section 2") {
		t.Errorf("errs %v", errs)
	}
}

func TestTrimFrontmatter(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ in, want string }{
		"drops the Claude Code keys": {
			"---\nname: a\nargument-hint: <x>\ndescription: b\nuser-invocable: false\n---\nBody.\n",
			"---\nname: a\ndescription: b\n---\nBody.\n",
		},
		"drops a list": {
			"---\nname: a\nallowed-tools:\n  - Bash(git diff *)\ndescription: b\n---\n",
			"---\nname: a\ndescription: b\n---\n",
		},
		"keeps a long line whole": {
			"---\nname: a\ndescription: " + strings.Repeat("word ", 60) + "end\n---\n",
			"---\nname: a\ndescription: " + strings.Repeat("word ", 60) + "end\n---\n",
		},
		"no frontmatter stays":     {"Body.\n", "Body.\n"},
		"broken frontmatter stays": {"---\nname: [a\n---\n", "---\nname: [a\n---\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := trimFrontmatter(tc.in); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLint(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		line string
		want string // "" when the line is clean
	}{
		"shell block":         {"Today: !`date`", "shell block"},
		"arguments":           {"Use $ARGUMENTS here", "$ARGUMENTS"},
		"kit command":         {"Run `kit scratch-dir` first", "kit command"},
		"kit.yml":             {"The lists live in kit.yml", "kit.yml"},
		"rule file":           {"See rules/verify.md", "rule file"},
		"skills path":         {"See skills/pr/SKILL.md", "skills/ path"},
		"AskUserQuestion":     {"Ask with AskUserQuestion", "AskUserQuestion"},
		"gh command":          {"Run gh pr view", "gh command"},
		"git command":         {"Run git log", "git command"},
		"env var":             {"Set CLAUDE_PLUGIN_ROOT", "CLAUDE_ env var"},
		"hyphenated skill":    {"Follow test-patterns for this", "skill not in the bundle"},
		"code-spanned skill":  {"Load `pr` first", "skill not in the bundle"},
		"typed skill":         {"Run /audit now", "skill not in the bundle"},
		"typed in code":       {"So `/audit debt` finds it", "skill not in the bundle"},
		"the word audit":      {"An audit of the pr is fine", ""},
		"a bundled skill":     {"Use the write skill and slop-patterns", ""},
		"the kit in prose":    {"The kit has a claude-kit config, and git history", ""},
		"a longer word":       {"Follow my-test-patterns-thing", ""},
		"the rules skill":     {"See the kit-rules skill", ""},
		"a sentence on rules": {"These rules apply", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := &bundle{
				files:  map[string][]byte{"skills/a/x.md": []byte("clean\n" + tc.line + "\n"), "skills/a/run.sh": []byte("git log\n")},
				others: []string{"audit", "pr", "test-patterns"},
			}
			errs := b.lint()
			if tc.want == "" {
				if len(errs) > 0 {
					t.Errorf("flagged a clean line: %v", errs)
				}
				return
			}
			if len(errs) == 0 || !strings.Contains(errs[0].Error(), "skills/a/x.md:2: "+tc.want) {
				t.Errorf("errs %v, want skills/a/x.md:2: %s", errs, tc.want)
			}
		})
	}
}

func TestEveryKitCommandIsForbidden(t *testing.T) {
	t.Parallel()
	for _, name := range kitNames() {
		if !forbidden["kit command"].MatchString("run kit " + name) {
			t.Errorf("kit %s isn't forbidden", name)
		}
	}
}

func TestBundleRunsTheTransforms(t *testing.T) {
	t.Parallel()
	root, cfg := checkout(t)
	testutil.Put(t, root, "skills/alpha/SKILL.md", "---\nname: alpha\ndescription: Use for alpha work.\nargument-hint: <x>\n---\n\n<!-- chat: skip -->\n\nDropped.\n\n<!-- chat: end -->\n\nSee `rules/change.md` section 1.\n")
	var stdout, stderr bytes.Buffer
	if code := Run(root, cfg, []string{"chat"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr %s", code, &stderr)
	}
	got := unzip(t, filepath.Join(root, "bin", "kit-chat-1.2.3.zip"))["skills/alpha/SKILL.md"]
	want := "---\nname: alpha\ndescription: Use for alpha work.\n---\n\nSee the kit-rules skill, Change section 1.\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRulesBecomeASkill(t *testing.T) {
	t.Parallel()
	root, cfg := checkout(t)
	testutil.Put(t, root, "rules/output.md", "# Output\n\n<!-- chat: skip -->\n\nDropped.\n\n<!-- chat: end -->\n\n## 4. External\n\nSee `rules/change.md` section 1.\n")
	cfg.ChatBundle.Rules = []string{"rules/output.md", "rules/change.md"}
	var stdout, stderr bytes.Buffer
	if code := Run(root, cfg, []string{"chat"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr %s", code, &stderr)
	}
	got := unzip(t, filepath.Join(root, "bin", "kit-chat-1.2.3.zip"))["skills/kit-rules/SKILL.md"]
	want := "---\nname: kit-rules\ndescription: Load for coding.\n---\n\nThe head.\n\n# Output\n\n## 4. External\n\nSee the kit-rules skill, Change section 1.\n\n# Change\n\n## 1. One\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The real rules, with the real head, as chat gets them. Hits in other
// files aren't this test's: the skills have their own markers.
func TestRealRulesSkillIsClean(t *testing.T) {
	t.Parallel()
	root := "../../.."
	c := testutil.KitConfig(t).ChatBundle
	b, err := build(root, c)
	if err != nil {
		t.Fatal(err)
	}
	const file = "skills/kit-rules/SKILL.md"
	for _, err := range slices.Concat(b.transform(c), b.lint()) {
		if strings.HasPrefix(err.Error(), file) || strings.HasPrefix(err.Error(), "rules/") {
			t.Error(err)
		}
	}
	for _, err := range skillProblems(rulesSkill, b.files[file]) {
		t.Error(err)
	}
}

func TestBuildProblems(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		mutate func(*testing.T, string, *config.Config)
		want   string
	}{
		"unclosed marker in a skill": {func(t *testing.T, root string, _ *config.Config) {
			testutil.Put(t, root, "skills/alpha/reference/more.md", "<!-- chat: skip -->\n")
		}, "skills/alpha/reference/more.md: line 1: marker never closed"},
		"unclosed marker in a rule": {func(t *testing.T, root string, _ *config.Config) {
			testutil.Put(t, root, "rules/change.md", "# Change\n<!-- chat: skip -->\n")
		}, "rules/change.md: line 2"},
		"rule with no title": {func(t *testing.T, root string, _ *config.Config) {
			testutil.Put(t, root, "rules/change.md", "text\n")
		}, "rules/change.md: no # title"},
		"missing rule": {func(_ *testing.T, _ string, cfg *config.Config) {
			cfg.ChatBundle.Rules = []string{"rules/gone.md"}
		}, "rules/gone.md"},
		"no rules": {func(_ *testing.T, _ string, cfg *config.Config) {
			cfg.ChatBundle.Rules = nil
		}, "chat_bundle.rules"},
		"no rules head": {func(t *testing.T, root string, _ *config.Config) {
			if err := os.Remove(filepath.Join(root, rulesHeadPath)); err != nil {
				t.Fatal(err)
			}
		}, "kit-rules.md"},
		"a rules skill too long": {func(t *testing.T, root string, _ *config.Config) {
			testutil.Put(t, root, "rules/change.md", "# Change\n"+strings.Repeat("line\n", maxSkillLines))
		}, "skills/kit-rules/SKILL.md: 500 lines"},
		"section a marker dropped": {func(t *testing.T, root string, _ *config.Config) {
			testutil.Put(t, root, "rules/change.md", "# Change\n\n<!-- chat: skip -->\n\n## 1. One\n\n<!-- chat: end -->\n")
			testutil.Put(t, root, "skills/alpha/reference/more.md", "`rules/change.md` section 1\n")
		}, "links rules/change.md section 1"},
		"lint hit": {func(t *testing.T, root string, _ *config.Config) {
			testutil.Put(t, root, "skills/alpha/reference/more.md", "ok\nRun `kit scratch-dir`.\n")
		}, "skills/alpha/reference/more.md:2: kit command"},
		"a left-out skill by name": {func(t *testing.T, root string, _ *config.Config) {
			testutil.Put(t, root, "skills/alpha/reference/more.md", "See `beta`.\n")
		}, "skill not in the bundle"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, cfg := checkout(t)
			tc.mutate(t, root, cfg)
			var stdout, stderr bytes.Buffer
			if code := Run(root, cfg, []string{"chat", "--check"}, &stdout, &stderr); code != 1 {
				t.Errorf("exit %d, want 1, stderr %s", code, &stderr)
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Errorf("stderr %q lacks %q", &stderr, tc.want)
			}
		})
	}
}

func TestSkillsDirMissingFails(t *testing.T) {
	t.Parallel()
	root, cfg := checkout(t)
	if err := os.RemoveAll(filepath.Join(root, "skills")); err != nil {
		t.Fatal(err)
	}
	cfg.ChatBundle.Skills = nil
	var stdout, stderr bytes.Buffer
	if code := Run(root, cfg, []string{"chat", "--check"}, &stdout, &stderr); code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
}
