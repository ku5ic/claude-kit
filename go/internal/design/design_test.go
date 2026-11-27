package design

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestContrast(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args   []string
		status int
		want   []string
	}{
		{[]string{"#767676", "#fff"}, 0, []string{"4.54:1", "AA normal text (4.5:1): pass", "AAA normal text (7:1): fail", "AAA large text (4.5:1): pass"}},
		{[]string{"#fff", "#767676"}, 0, []string{"4.54:1"}}, // order doesn't matter
		{[]string{"000", "rgb(255, 255, 255)"}, 0, []string{"21.00:1", "AAA normal text (7:1): pass"}},
		{[]string{"#777", "#fff"}, 0, []string{"4.47:1", "AA normal text (4.5:1): fail", "AA large text (3:1): pass"}},
		{[]string{"rgb(0 0 0)", "#ffffff"}, 0, []string{"21.00:1"}},
		{[]string{"#fff"}, 2, []string{"usage: kit contrast"}},
		{nil, 2, []string{"usage: kit contrast"}},
		{[]string{"red", "#fff"}, 2, []string{`"red": not #rgb`}},
		{[]string{"#12345g", "#fff"}, 2, []string{"not #rgb"}},
		{[]string{"rgb(300, 0, 0)", "#fff"}, 2, []string{"isn't 0 to 255"}},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			if got := Contrast(tc.args, &out, &out); got != tc.status {
				t.Errorf("status %d, want %d", got, tc.status)
			}
			for _, w := range tc.want {
				if !strings.Contains(out.String(), w) {
					t.Errorf("output lacks %q:\n%s", w, out.String())
				}
			}
		})
	}
}

func TestUndefinedTokens(t *testing.T) {
	t.Parallel()
	project := func(t *testing.T, files map[string]string) string {
		t.Helper()
		root := testutil.Physical(t, t.TempDir())
		testutil.Git(t, root, "init", "-q")
		for name, body := range files {
			testutil.Write(t, filepath.Join(root, name), body)
		}
		return root
	}
	tw := map[string]string{
		"src/app.css": "@import \"tailwindcss\";\n@theme {\n  --color-brand-500: oklch(60% 0.2 250);\n}\n:root { --gap: 1rem; }\n",
	}
	for _, tc := range []struct {
		name          string
		files         map[string]string
		before, after string
		want          []string
	}{
		{"an undefined var and an undefined shade", tw, "",
			`<div className="p-[var(--gap)] bg-brand-500 text-brand-600 hover:bg-primary-700" style={{color: "var(--gap-lg)"}} />`,
			[]string{"var(--gap-lg)", "--color-brand-600", "--color-primary-700"}},
		{"a var in a namespace only a dependency sets", tw, "", `<p style={{c: "var(--accent-9)", m: "var(--mantine-color-blue-6)"}} />`, nil},
		{"defined, default, fallback, and Tailwind's own", tw, "",
			`<div className="bg-red-500 border-x-mauve-200 text-shadow-sky-300 bg-brand-500/50" style={{gap: "var(--gap)", c: "var(--other, red)", w: "var(--spacing)", k: "var(--color-blue-500)"}} />`,
			nil},
		{"a var the edit defines itself", tw, "", ".card { --accent: red; color: var(--accent); }", nil},
		{"a token already there", tw, `<p className="bg-primary-700" style={{c: "var(--accent)"}} />`, `<p className="bg-primary-700 m-2" style={{c: "var(--accent)"}} />`, nil},
		{"a JS config: shades aren't checked", map[string]string{"src/app.css": tw["src/app.css"], "tailwind.config.ts": "export default {}"}, "",
			`<div className="bg-primary-700" />`, nil},
		{"no Tailwind: only vars are checked", map[string]string{"src/app.css": ":root { --gap: 1rem; }"}, "",
			`<div className="bg-primary-700" style={{c: "var(--gap-lg)"}} />`, []string{"var(--gap-lg)"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := project(t, tc.files)
			got := UndefinedTokens("", filepath.Join(root, "src", "Card.tsx"), tc.before, tc.after)
			if !slices.Equal(got, tc.want) {
				t.Errorf("UndefinedTokens = %q, want %q", got, tc.want)
			}
		})
	}
	t.Run("not a UI file", func(t *testing.T) {
		t.Parallel()
		root := project(t, tw)
		if got := UndefinedTokens("", filepath.Join(root, "main.go"), "", `var(--x)`); got != nil {
			t.Errorf("UndefinedTokens = %q, want none", got)
		}
	})
}

// readProject answers a root from its memo after the first read, a failed
// one too: what changes on disk after it isn't seen.
func TestReadProjectReadsARootOnce(t *testing.T) {
	t.Parallel()
	listed := testutil.Physical(t, t.TempDir())
	testutil.Git(t, listed, "init", "-q")
	testutil.Write(t, filepath.Join(listed, "app.css"), ":root { --gap: 1rem; }\n")
	if p, ok := readProject(listed, ""); !ok || !p.defined["--gap"] {
		t.Fatalf("first read = %v, %v; want --gap defined", p.defined, ok)
	}
	if err := os.RemoveAll(filepath.Join(listed, ".git")); err != nil {
		t.Fatal(err)
	}
	if p, ok := readProject(listed, ""); !ok || !p.defined["--gap"] {
		t.Errorf("second read = %v, %v; want the memo's --gap", p.defined, ok)
	}

	unlisted := testutil.Physical(t, t.TempDir())
	if _, ok := readProject(unlisted, ""); ok {
		t.Fatal("a root git can't list read ok")
	}
	testutil.Git(t, unlisted, "init", "-q")
	if _, ok := readProject(unlisted, ""); ok {
		t.Error("a failed read was retried")
	}
}

// scanProject reads again only a UI file whose size or mtime changed since
// the last scan: a changed file redefines its tokens, and an unchanged one
// keeps what the cache holds, even when its bytes were swapped behind the
// same size and mtime.
func TestScanProjectRereadsOnlyChangedFiles(t *testing.T) {
	t.Parallel()
	root := testutil.Physical(t, t.TempDir())
	cacheDir := t.TempDir()
	testutil.Git(t, root, "init", "-q")
	testutil.Write(t, filepath.Join(root, "app.css"), "@import \"tailwindcss\";\n")
	testutil.Write(t, filepath.Join(root, "vars.css"), ":root { --gap: 1rem; }\n")
	if p, ok := scanProject(root, cacheDir); !ok || !p.tailwind || !p.defined["--gap"] {
		t.Fatalf("first scan = %+v, %v", p, ok)
	}

	app := filepath.Join(root, "app.css")
	info, err := os.Stat(app)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, app, "@import \"tailwindxss\";\n")
	if err := os.Chtimes(app, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, filepath.Join(root, "vars.css"), ":root { --pad: 2rem; --x: 0; }\n")

	p, ok := scanProject(root, cacheDir)
	if !ok || !p.tailwind {
		t.Errorf("an unchanged Tailwind file lost its flag: %+v, %v", p, ok)
	}
	if !p.defined["--pad"] || p.defined["--gap"] {
		t.Errorf("a changed file kept its old tokens: %v", p.defined)
	}
}

// An index the scan can't trust costs a full read, never a wrong answer:
// one that doesn't decode, one an older kit wrote, and a deleted file's
// entry, which the next write drops.
func TestScanProjectDistrustsAStaleIndex(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (root, cacheDir string) {
		root, cacheDir = testutil.Physical(t, t.TempDir()), t.TempDir()
		testutil.Git(t, root, "init", "-q")
		testutil.Write(t, filepath.Join(root, "vars.css"), ":root { --gap: 1rem; }\n")
		return root, cacheDir
	}
	// stale is an index entry for vars.css that matches its size and mtime
	// but names a token the file doesn't define.
	stale := func(t *testing.T, root string) []byte {
		info, err := os.Stat(filepath.Join(root, "vars.css"))
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(map[string]fileTokens{"vars.css": {Size: info.Size(), MTime: info.ModTime().UnixNano(), Defined: []string{"--stale"}}}); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	t.Run("an index that doesn't decode", func(t *testing.T) {
		t.Parallel()
		root, cacheDir := setup(t)
		testutil.Write(t, indexFile(cacheDir, root), "not gob")
		if p, ok := scanProject(root, cacheDir); !ok || !p.defined["--gap"] {
			t.Errorf("scan = %+v, %v, want --gap from a full read", p, ok)
		}
	})
	t.Run("an index an older kit wrote", func(t *testing.T) {
		t.Parallel()
		root, cacheDir := setup(t)
		testutil.Write(t, filepath.Join(cacheDir, cache.Tokens, cache.RootKey(root)+".gob"), string(stale(t, root)))
		if p, _ := scanProject(root, cacheDir); p.defined["--stale"] || !p.defined["--gap"] {
			t.Errorf("an unversioned index answered: %v", p.defined)
		}
	})
	t.Run("a deleted file's entry", func(t *testing.T) {
		t.Parallel()
		root, cacheDir := setup(t)
		testutil.Write(t, filepath.Join(root, "app.css"), "a {}\n")
		scanProject(root, cacheDir)
		if err := os.Remove(filepath.Join(root, "vars.css")); err != nil {
			t.Fatal(err)
		}
		if p, _ := scanProject(root, cacheDir); p.defined["--gap"] {
			t.Errorf("a deleted file's tokens are still defined: %v", p.defined)
		}
		if _, kept := loadIndex(indexFile(cacheDir, root))["vars.css"]; kept {
			t.Error("the index still holds the deleted file")
		}
	})
}
