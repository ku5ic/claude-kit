package slop

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// fileRules scans after, as written over before at path, and returns the
// line:rule of each hit.
func fileRules(t *testing.T, d *Detector, path, before, after string) []string {
	t.Helper()
	segs, _, ok := Segments(path, before, after)
	if !ok {
		t.Fatalf("%s: no reader", path)
	}
	var got []string
	for _, h := range d.Scan(segs, "file").Hits {
		got = append(got, fmt.Sprintf("%d:%s", h.Line, h.Rule))
	}
	return got
}

func lines(ls ...string) string { return strings.Join(ls, "\n") }

func TestComments(t *testing.T) {
	t.Parallel()
	d := detector(t)
	cases := []struct {
		name, path, text string
		want             []string
	}{
		{"go line comment", "a.go", lines(
			"// Run serves as the entry point.",
			`x := "// serves as the string"`,
			`url := http://x.dev // plain`,
		), []string{"1:copula"}},
		{"go block comment", "a.go", lines(
			"/* The cache",
			"   serves as the store. */",
			"y := 2 /* stands as a marker */ + 1",
		), []string{"2:copula", "3:copula"}},
		{"code span in a comment", "a.go", "// The regexp matches `serves as a`.", nil},
		{"python docstring and hash", "a.py", lines(
			`def f():`,
			`    """Load the file.`,
			`    Reads it once, ensuring the cache stays warm."""`,
			`    s = "# serves as the"  # it serves as the key`,
		), []string{"3:ing-tail", "4:copula"}},
		{"shell", "a.sh", lines(
			`echo $# ${#x}`,
			`# I hope this helps`,
		), []string{"2:hope-helps"}},
		{"shebang without an extension", "bin/kit", lines(
			"#!/usr/bin/env bash",
			`echo "I hope this helps"`,
			"# I hope this helps",
		), []string{"3:hope-helps"}},
		{"yaml value is not prose", "a.yml", lines(
			`match: '(?i)\bserves as the\b'`,
			`fix: is # serves as the comment`,
		), []string{"2:copula"}},
		{"jsx comment", "a.tsx", `return <div>{/* serves as the root */}</div>`, []string{"1:copula"}},
		{"html comment", "a.html", "<!-- I hope this helps -->", []string{"1:hope-helps"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := fileRules(t, d, c.path, "", c.text); !slices.Equal(got, c.want) {
				t.Errorf("hits = %v, want %v", got, c.want)
			}
		})
	}
}

func TestUI(t *testing.T) {
	t.Parallel()
	d := detector(t)
	cases := []struct {
		name, path, text string
		want             []string
	}{
		{"lorem ipsum", "Hero.tsx", `<p>Lorem ipsum dolor sit amet</p>`, []string{"1:ui-placeholder"}},
		{"default gradient", "Hero.tsx", `<div className="bg-gradient-to-r from-purple-500 to-blue-500">`, []string{"1:ui-default-gradient"}},
		{"gradient text", "Hero.vue", `<h1 class="bg-clip-text text-transparent">`, []string{"1:ui-gradient-text"}},
		{"marketing verb", "Hero.jsx", `<h2>Supercharge your workflow</h2>`, []string{"1:ui-marketing"}},
		{"emoji in a button", "Cta.tsx", "<button>Start " + string(rune(0x1F680)) + "</button>", []string{"1:ui-emoji-label"}},
		// False positives from the design reference: tokens and one-sided
		// palette classes are choices, not tells.
		{"brand token", "theme.css", `--color-brand: #7c3aed;`, nil},
		{"elevation variant", "Card.tsx", `<Card variant="elevated">`, nil},
		{"one palette class", "Badge.tsx", `<span className="bg-purple-500 rounded-lg shadow-md">`, nil},
		{"prose rules skip code", "Card.tsx", `const label = "it serves as the entry point";`, nil},
		{"ui rules skip non-UI files", "theme.go", `// from-purple-500 to-blue-500`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := fileRules(t, d, c.path, "", c.text); !slices.Equal(got, c.want) {
				t.Errorf("hits = %v, want %v", got, c.want)
			}
		})
	}
}

func TestAddedLines(t *testing.T) {
	t.Parallel()
	d := detector(t)
	old := lines(
		"# Notes",
		"",
		"The hook serves as the entry point.",
		"",
		"```text",
		"example",
		"```",
	)
	cases := []struct {
		name, after string
		want        []string
	}{
		{"old slop doesn't block a new line", strings.Replace(old, "# Notes", "# Notes\n\nA plain line.", 1), nil},
		{"a new tell is found", old + "\n\nI hope this helps.", []string{"9:hope-helps"}},
		{"a tell inside an existing fence", strings.Replace(old, "example", "example\nI hope this helps.", 1), nil},
		{"a duplicated old line counts as added", old + "\n\nThe hook serves as the entry point.", []string{"9:copula"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := fileRules(t, d, "notes.md", old, c.after); !slices.Equal(got, c.want) {
				t.Errorf("hits = %v, want %v", got, c.want)
			}
		})
	}
}

func TestAllowMarkerInCode(t *testing.T) {
	t.Parallel()
	d := detector(t)
	before := lines(
		"// unslop: allow quoting the research",
		"var tell = 1",
	)
	after := lines(
		"// unslop: allow quoting the research",
		"// I hope this helps",
		"var tell = 1",
		"// I hope this helps",
	)
	segs, allows, _ := Segments("a.go", before, after)
	if len(allows) != 0 {
		t.Errorf("an old marker logged again: %+v", allows)
	}
	var got []int
	for _, h := range d.Scan(segs, "file").Hits {
		got = append(got, h.Line)
	}
	if !slices.Equal(got, []int{4}) {
		t.Errorf("hit lines = %v, want [4]", got)
	}
}

func TestNoReader(t *testing.T) {
	t.Parallel()
	if _, _, ok := Segments("logo.png", "", "x"); ok {
		t.Error("png has a reader")
	}
}
