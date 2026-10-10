package explain

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/sources"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// stopRepo is a repo whose package.json runs fakelint on .ts files through
// lint-staged, classified as a lint check with a stub classifier, with
// a.ts, b.ts, and notes.md uncommitted. It returns the kit's paths, the
// config, and the repo's root.
func stopRepo(t *testing.T) (config.Paths, *config.Config, string) {
	t.Helper()
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(tmp, "repo")
	testutil.FakeTool(t, filepath.Join(root, "node_modules/.bin/fakelint"), filepath.Join(tmp, "calls"), "")
	testutil.Git(t, root, "init", "-q")
	testutil.Git(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init")
	testutil.Put(t, root, ".gitignore", "node_modules\n")
	testutil.Put(t, root, "package.json", `{"lint-staged":{"*.ts":"fakelint --check"}}`)
	for _, f := range []string{"a.ts", "b.ts", "notes.md"} {
		testutil.Put(t, root, f, "x\n")
	}
	lint := sources.Entry{Source: "lint-staged", File: "package.json", Name: "*.ts", Dir: ".", Body: sources.Body{Text: "fakelint --check"}, Files: []string{"*.ts"}, PassFiles: true, Stage: "pre-commit"}
	envelope, err := json.Marshal(map[string]any{"is_error": false, "subtype": "success", "structured_output": map[string]any{
		"entries":  []any{map[string]any{"id": gapfill.Key(lint), "role": "check", "kind": "lint", "mutates": false}},
		"managers": []any{}, "proposals": []any{}}})
	if err != nil {
		t.Fatal(err)
	}
	testutil.Put(t, tmp, "answer.json", string(envelope))
	cfg := testutil.KitConfig(t)
	cfg.Classifier = testutil.Replayer(t, filepath.Join(tmp, "answer.json"))
	paths := config.Paths{Home: filepath.Join(tmp, "kit")}
	gapfill.Run(cfg, gapfill.Options{Root: root, CacheDir: paths.CacheDir(), Entries: sources.Entries(cfg, root, project.Subprojects(root)), Ask: true, Timeout: time.Minute})
	return paths, cfg, root
}

// explainStop is kit explain stop with files, run in root.
func explainStop(t *testing.T, paths config.Paths, cfg *config.Config, root string, files ...string) string {
	t.Helper()
	var out, stderr strings.Builder
	if status := Run(paths, cfg, root, append([]string{"stop"}, files...), &out, &stderr); status != 0 {
		t.Fatalf("status %d: %s", status, stderr.String())
	}
	return out.String()
}

func TestStop(t *testing.T) {
	t.Parallel()
	has := func(t *testing.T, out string, subs ...string) {
		t.Helper()
		for _, s := range subs {
			if !strings.Contains(out, s) {
				t.Errorf("output lacks %q:\n%s", s, out)
			}
		}
	}

	t.Run("kit explain stop names what claims a file, the command, and the binary's source", func(t *testing.T) {
		t.Parallel()
		paths, cfg, root := stopRepo(t)
		a, notes := filepath.Join(root, "a.ts"), filepath.Join(root, "notes.md")
		has(t, explainStop(t, paths, cfg, root, a, notes),
			"lint (package.json: *.ts)\n  runs in  .\n  file     a.ts\n  command  fakelint --check a.ts\n  source   "+filepath.Join(root, "node_modules/.bin/fakelint")+" (local)\n  blocks   findings on changed lines\n",
			"unclaimed  "+notes)
	})
	t.Run("with no files, kit explain stop takes the working tree's changes", func(t *testing.T) {
		t.Parallel()
		paths, cfg, root := stopRepo(t)
		has(t, explainStop(t, paths, cfg, root), "  file     a.ts\n  file     b.ts\n")
	})
	// The hooks are silent on a pass; their last reports are kept for this.
	t.Run("with no files, it prints the last stop-checks and per-edit runs first", func(t *testing.T) {
		t.Parallel()
		paths, cfg, root := stopRepo(t)
		for path, report := range map[string]string{
			cache.StopReport(paths.CacheDir(), root): "PASS lint (package.json: *.ts)\n",
			cache.EditReport(paths.CacheDir(), root): "SKIP fix (evidence .prettierrc) (dprint declared in package.json but not installed)\n",
		} {
			testutil.Put(t, filepath.Dir(path), filepath.Base(path), report)
		}
		has(t, explainStop(t, paths, cfg, root),
			"last stop-checks run:\nPASS lint (package.json: *.ts)\n\n",
			"last per-edit run:\nSKIP fix (evidence .prettierrc) (dprint declared in package.json but not installed")
	})
}
