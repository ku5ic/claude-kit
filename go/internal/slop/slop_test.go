package slop

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func detector(t *testing.T, disabled ...string) *Detector {
	t.Helper()
	cfg := testutil.KitConfig(t)
	cfg.DisabledRules = disabled
	d, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// rules is the rule id of each hit, in order.
func rules(r Result) []string {
	var ids []string
	for _, h := range r.Hits {
		ids = append(ids, h.Rule)
	}
	return ids
}

func scan(d *Detector, text, surface string) Result {
	segs, _ := Prose(text)
	return d.Scan(segs, surface)
}

func TestTiers(t *testing.T) {
	t.Parallel()
	d := detector(t)
	cases := []struct {
		name, text, surface string
		want                []string
	}{
		// Tier 1: any single hit.
		{"hope it helps", "That's the fix. I hope this helps!", "chat", []string{"hope-helps"}},
		{"offer more", "Let me know if you want the tests too.", "chat", []string{"offer-more"}},
		{"stock opener", "Certainly! Here's the patch.", "chat", []string{"stock-opener"}},
		{"opener in a list item", "- Great question, it depends.", "file", []string{"stock-opener"}},
		{"absolutely right", "You're absolutely right, it was the cache.", "chat", []string{"absolutely-right"}},
		{"leaked citation", "Sales rose 4% contentReference[oaicite:1]", "file", []string{"citation-leak"}},
		{"ai utm", "See [docs](https://x.dev/a?utm_source=chatgpt.com).", "file", []string{"ai-utm"}},
		{"placeholder date", "Released on 2026-XX-XX by [Your Name].", "file", []string{"placeholder", "placeholder"}},
		// Tier 2: structural.
		{"copula", "The hook serves as the entry point.", "file", []string{"copula"}},
		{"not only", "It not only caches but also logs.", "file", []string{"not-only"}},
		{"ing tail", "The gate reruns, ensuring consistent output.", "file", []string{"ing-tail"}},
		{"significance", "Caching plays a crucial role in the gate.", "file", []string{"significance"}},
		{"vague connection", "He was associated with the review hook.", "file", []string{"vague-connection"}},
		{"corrective framing", "The count should be treated as a floor.", "chat", []string{"corrective-framing"}},
		{"generic reason in a commit", "Refactor the loader to improve readability.", "commit", []string{"generic-reason"}},
		{"generic reason in a file", "Refactor the loader to improve readability.", "file", nil},
		{"preserving in a PR", "Split the parser while preserving behavior.", "pr", []string{"while-preserving"}},
		{"addressed feedback", "Addressed review feedback.", "commit", []string{"addressed-feedback"}},
		// Tier 3 alone never reports.
		{"one rather than", "It runs the file form rather than the tree.", "file", nil},
		{"one era word", "A robust estimator of the median.", "file", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := rules(scan(d, c.text, c.surface)); !slices.Equal(got, c.want) {
				t.Errorf("hits = %v, want %v", got, c.want)
			}
		})
	}
}

func TestNotTells(t *testing.T) {
	t.Parallel()
	d := detector(t)
	texts := map[string]string{
		"quoted tell":         `Avoid "serves as the entry point"; say "is".`,
		"curly quoted tell":   "Avoid " + string(rune(0x201C)) + "I hope this helps" + string(rune(0x201D)) + " at the end.",
		"code span":           "The rule matches `serves as a` and `I hope this helps`.",
		"double backtick":     "Write ``a `serves as a` b`` literally.",
		"fenced block":        "Before.\n\n```text\nI hope this helps\n```\n\nAfter.",
		"frontmatter":         "---\ndescription: I hope this helps\n---\n\nBody.",
		"blockquote":          "> Certainly! I hope this helps.\n\nOur reply.",
		"of course mid-text":  "Of course, the cache is cold on the first run.",
		"certainly in prose":  "That's certainly the slower path.",
		"technical relation":  "The token associated with the session expires.",
		"acts as":             "nginx acts as a reverse proxy.",
		"preserving in files": "The formatter runs while preserving comments.",
	}
	for name, text := range texts {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := rules(scan(d, text, "file")); len(got) != 0 {
				t.Errorf("hits = %v, want none", got)
			}
		})
	}
}

func TestDensity(t *testing.T) {
	t.Parallel()
	d := detector(t)
	dense := "We delve into the pivotal, multifaceted landscape of caching."
	r := scan(d, dense, "file")
	if !r.Dense {
		t.Fatalf("not dense: score %.1f, hits %v", r.Score, rules(r))
	}
	if got := rules(r); len(got) != 4 || got[0] != "vocabulary" {
		t.Errorf("hits = %v, want 4 vocabulary hits", got)
	}
	for _, h := range r.Hits {
		if d.Blocks(h) {
			t.Errorf("tier 3 hit %v blocks", h)
		}
	}
	// The same words spread over a long text stay under the threshold.
	sparse := dense + strings.Repeat(" The gate runs each check on the changed files and exits.", 20)
	if r := scan(d, sparse, "file"); r.Dense || len(r.Hits) != 0 {
		t.Errorf("sparse text dense %v, hits %v", r.Dense, rules(r))
	}
}

func TestDisabledRules(t *testing.T) {
	t.Parallel()
	d := detector(t, "slop-copula", "slop-vocabulary")
	if got := rules(scan(d, "The hook serves as the entry point.", "file")); len(got) != 0 {
		t.Errorf("disabled copula still hits: %v", got)
	}
	if r := scan(d, "We delve into the pivotal, multifaceted landscape.", "file"); r.Dense {
		t.Errorf("disabled vocabulary still dense: %v", rules(r))
	}
}

func TestBlocks(t *testing.T) {
	t.Parallel()
	d := detector(t)
	r := scan(d, "The hook serves as the entry point. I hope this helps.", "chat")
	got := d.Blocking(r)
	if len(got) != 1 || got[0].Rule != "hope-helps" {
		t.Errorf("blocking = %+v, want only tier 1 hope-helps", got)
	}
}

func TestProse(t *testing.T) {
	t.Parallel()
	text := strings.Join([]string{
		"# Title", // 1
		"<!-- unslop: allow quoting the research -->", // 2
		"",                                        // 3
		"The hook serves as the entry point.",     // 4: exempt
		"The next line counts.",                   // 5
		"<!-- unslop: allow -->",                  // 6: no reason, no marker
		"It has `code` and \"a quote\" in it.",    // 7
		"Name `unslop: allow <reason>` in prose.", // 8: a mention
	}, "\n")
	segs, allows := Prose(text)
	if len(allows) != 1 || allows[0] != (Allow{2, "quoting the research"}) {
		t.Errorf("allows = %+v", allows)
	}
	var lines []int
	for _, s := range segs {
		lines = append(lines, s.Line)
	}
	if want := []int{1, 5, 6, 7, 8}; !slices.Equal(lines, want) {
		t.Errorf("lines = %v, want %v", lines, want)
	}
	if seven := segs[3].Text; strings.Contains(seven, "code") || strings.Contains(seven, "quote") {
		t.Errorf("code span or quote left in %q", seven)
	}
}

func TestStripCode(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"a `b` c":         "a   c",
		"a ``b ` c`` d":   "a   d",
		"unclosed `b c":   "unclosed `b c",
		"two `a` and `b`": "two   and  ",
	}
	for in, want := range cases {
		if got := stripCode(in); got != want {
			t.Errorf("stripCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRun(t *testing.T) {
	t.Parallel()
	cfg := testutil.KitConfig(t)
	dir := t.TempDir()
	put := func(name, body string) string {
		path := filepath.Join(dir, name)
		testutil.Put(t, dir, name, body)
		return path
	}
	tell := put("tell.md", "Done. I hope this helps.\n")
	soft := put("soft.md", "The hook serves as the entry point.\n")
	clean := put("clean.md", "The hook is the entry point.\n")
	code := put("main.go", "package main\n")
	skipped := put("testdata/x.md", "I hope this helps.\n")

	cases := []struct {
		name   string
		args   []string
		stdin  string
		status int
		out    string
		errOut string
	}{
		{"tier 1 blocks", []string{tell}, "", 1, `:1: T1 hope-helps "I hope this helps" -> cut it`, ""},
		{"tier 2 is logged, not blocked", []string{soft}, "", 0, `:1: T2 copula "serves as the" -> is`, ""},
		{"clean", []string{clean}, "", 0, "", ""},
		{"no reader", []string{code}, "", 0, "", "not scanned"},
		{"skipped", []string{skipped}, "", 0, "", "skipped (slop.skip)"},
		{"stdin", []string{"--surface", "commit", "-"}, "Addressed review feedback.\n", 0, "-:1: T2 addressed-feedback", ""},
		{"stdin needs a surface", []string{"-"}, "x", 2, "", "stdin needs --surface"},
		{"unknown surface", []string{"--surface=email", clean}, "", 2, "", "is no surface"},
		{"no file", nil, "", 2, "", "no file"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var out, errOut bytes.Buffer
			status := Run(cfg, c.args, strings.NewReader(c.stdin), &out, &errOut)
			if status != c.status {
				t.Errorf("status %d, want %d; stdout %q stderr %q", status, c.status, out.String(), errOut.String())
			}
			if !strings.Contains(out.String(), c.out) || c.out == "" && out.Len() > 0 {
				t.Errorf("stdout %q, want %q", out.String(), c.out)
			}
			if !strings.Contains(errOut.String(), c.errOut) {
				t.Errorf("stderr %q, want %q", errOut.String(), c.errOut)
			}
		})
	}
}

// TestShippedLexicon guards the shipped kit.yml: every rule compiles, so
// New never fails open in a hook.
func TestShippedLexicon(t *testing.T) {
	t.Parallel()
	cfg, warnings, err := config.Load(config.Paths{Base: "../../../kit.yml", Overlay: filepath.Join(t.TempDir(), "none.yml")})
	if err != nil || len(warnings) > 0 {
		t.Fatalf("err %v, warnings %v", err, warnings)
	}
	if _, err := New(cfg); err != nil {
		t.Fatal(err)
	}
}
