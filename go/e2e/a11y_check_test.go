package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// kit a11y-check with a fake axe: digest format, and the exit codes for a
// missing axe (3) and a URL that does not answer (4).
func TestA11yCheck(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (k *Kit, repo, page string) {
		k = New(t)
		repo = filepath.Join(Physical(t, t.TempDir()), "repo")
		Mkdir(t, repo)
		k.Git(repo, "init", "-q", "-b", "main")
		k.Dir = repo
		Write(t, filepath.Join(repo, "page.html"), "<html></html>\n")
		return k, repo, "file://" + repo + "/page.html"
	}
	// A fake axe in the repo's node_modules/.bin that prints fixture results.
	fakeAxe := func(t *testing.T, repo string) {
		Stub(t, filepath.Join(repo, "node_modules", ".bin", "axe"), `cat <<'JSON'
[{"violations":[
  {"id":"color-contrast","impact":"serious","tags":["cat.color","wcag2aa","wcag143"],
   "nodes":[{"target":[".btn-primary"]},{"target":[".link"]}]},
  {"id":"region","impact":"moderate","tags":["cat.keyboard","best-practice"],
   "nodes":[{"target":[["my-card","p.note"]]}]}
]}]
JSON
`)
	}

	t.Run("digest: one line per rule with impact, wcag tags, node count, selector", func(t *testing.T) {
		t.Parallel()
		k, repo, page := setup(t)
		fakeAxe(t, repo)
		r := k.Run("", "a11y-check", page)
		r.Want(t, 0)
		lines := Lines(r.Output)
		if len(lines) < 3 {
			t.Fatalf("want 3 lines, got:\n%s", r.Output)
		}
		head := regexp.MustCompile(`^` + regexp.QuoteMeta("a11y-check: 2 violated rule(s) on "+page+" (raw: ") +
			`.*/\.claude/scratch/a11y-runtime-.*\.json\)$`)
		if !head.MatchString(lines[0]) {
			t.Fatalf("header %q does not match %s", lines[0], head)
		}
		if want := "color-contrast  serious  wcag2aa,wcag143  nodes=2  .btn-primary"; lines[1] != want {
			t.Errorf("line 2 %q, want %q", lines[1], want)
		}
		if want := "region  moderate  -  nodes=1  my-card p.note"; lines[2] != want {
			t.Errorf("line 3 %q, want %q", lines[2], want)
		}
		raw := strings.TrimSuffix(lines[0][strings.LastIndex(lines[0], "raw: ")+len("raw: "):], ")")
		body, err := os.ReadFile(raw)
		if err != nil {
			t.Fatal(err)
		}
		var results []struct{ Violations []any }
		if err := json.Unmarshal(body, &results); err != nil {
			t.Fatal(err)
		}
		if len(results) == 0 || len(results[0].Violations) != 2 {
			t.Errorf("raw %s: want 2 violations in the first result:\n%s", raw, body)
		}
	})

	t.Run("missing axe exits 3 with the install command", func(t *testing.T) {
		if _, err := exec.LookPath("axe"); err == nil {
			t.Skip("axe is installed globally")
		}
		k, _, page := setup(t)
		r := k.Run("", "a11y-check", page)
		r.Want(t, 3)
		r.Has(t, "npm install -D @axe-core/cli")
	})

	t.Run("a URL that does not answer exits 4 and names the start script", func(t *testing.T) {
		t.Parallel()
		k, repo, _ := setup(t)
		fakeAxe(t, repo)
		Write(t, filepath.Join(repo, "package.json"), `{"scripts":{"dev":"vite","storybook":"storybook dev"}}`+"\n")
		Touch(t, filepath.Join(repo, "pnpm-lock.yaml"))
		r := k.Run("", "a11y-check", "http://127.0.0.1:9/")
		r.Want(t, 4)
		r.Has(t, "Start it with: pnpm run storybook")
	})
}
