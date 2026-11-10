package hooks

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestGuardWeaken(t *testing.T) {
	t.Parallel()
	const test = "package a\n\nfunc TestA(t *testing.T) {\n\tif got != 1 {\n\t\tt.Errorf(\"got %d\", got)\n\t}\n}\n"
	edit := func(path, old, repl string) map[string]any {
		p := editPayload("Edit", path, "s", filepath.Dir(path))
		p["tool_input"].(map[string]any)["old_string"] = old
		p["tool_input"].(map[string]any)["new_string"] = repl
		return p
	}
	setup := func(t *testing.T) (*sandbox, string) {
		k := newSandbox(t)
		dir := repo(t, filepath.Join(k.home, "proj"))
		path := filepath.Join(dir, "pkg", "a_test.go")
		testutil.Write(t, path, test)
		return k, path
	}
	t.Run("an added skip blocks", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		r := k.run("guard-dispatch", edit(path, "\tif got", "\tt.Skip(\"flaky\")\n\tif got"))
		r.Want(t, 2)
		r.Has(t, "weakens a check in a_test.go", "skip", "kit: allow <reason>")
	})
	t.Run("an added skip with kit: allow goes through and is logged", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		k.run("guard-dispatch", edit(path, "\tif got", "\t// kit: allow needs docker\n\tt.Skip(\"no docker\")\n\tif got")).Allowed(t)
		if log := read(t, k.paths.LogFile(kitlog.Guards)); !strings.Contains(log, "weaken-allow") || !strings.Contains(log, "needs docker") {
			t.Errorf("guards log lacks the allow:\n%s", log)
		}
	})
	t.Run("a deleted assertion asks", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		r := k.run("guard-dispatch", edit(path, "\t\tt.Errorf(\"got %d\", got)\n", ""))
		r.Want(t, 0)
		r.Has(t, `"permissionDecision":"ask"`, "1 fewer assertion line")
	})
	t.Run("an ordinary edit goes through quietly", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		k.run("guard-dispatch", edit(path, "got != 1", "got != 2")).Empty(t)
	})
	t.Run("weaken in disabled_rules lets a skip through", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		k.overlay("disabled_rules: [weaken]\n")
		k.run("guard-dispatch", edit(path, "\tif got", "\tt.Skip(\"flaky\")\n\tif got")).Want(t, 0)
	})
}
