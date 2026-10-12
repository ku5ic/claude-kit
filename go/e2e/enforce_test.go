package e2e

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// Wiring for `kit enforce --list`: the readers' own cases are the sources
// package's tests.
func TestEnforceList(t *testing.T) {
	t.Parallel()
	t.Run("lists each entry with its source, from a subdirectory", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		root := k.Repo(filepath.Join(t.TempDir(), "repo"))
		Write(t, filepath.Join(root, "lefthook.yml"), "pre-commit:\n  commands:\n    lint:\n      glob: \"*.go\"\n      run: golangci-lint run {staged_files}\n")
		Write(t, filepath.Join(root, "turbo.json"), `{"tasks":{"test":{}}}`)
		Mkdir(t, filepath.Join(root, "src"))
		k.Dir = filepath.Join(root, "src")
		r := k.Run("", "enforce", "--list")
		r.Want(t, 0)
		if r.Stdout != "lefthook\tlefthook.yml\tpre-commit\t.\tlint\t*.go\tgolangci-lint run {files}\nturbo\tturbo.json\t-\t.\ttest\t-\tturbo run test\n" {
			t.Errorf("stdout:\n%s", r.Stdout)
		}
	})
	t.Run("anything but --list or classify is a usage error", func(t *testing.T) {
		t.Parallel()
		r := New(t).Run("", "enforce")
		r.Want(t, 2)
		r.Has(t, "usage: kit enforce --list | classify")
	})
}

// Wiring for `kit gates`: the run package's tests cover marks and restore.
func TestGates(t *testing.T) {
	t.Parallel()
	t.Run("anything but reset [--restore] is a usage error", func(t *testing.T) {
		t.Parallel()
		for _, args := range [][]string{{"gates"}, {"gates", "reset", "--force"}} {
			r := New(t).Run("", args...)
			r.Want(t, 2)
			r.Has(t, "usage: kit gates reset [--restore]")
		}
	})
	t.Run("with no gate seen changing files, reset says so", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		k.Dir = k.Repo(filepath.Join(t.TempDir(), "repo"))
		r := k.Run("", "gates", "reset", "--restore")
		r.Want(t, 0)
		r.Has(t, "no gate was seen changing files")
	})
}

// Wiring for `kit enforce classify`, with kit.yml's classifier key
// pointing at a stub: the gapfill package's tests cover the verdicts.
func TestEnforceClassify(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T, stub string) (*Kit, string) {
		k := New(t)
		root := k.Repo(filepath.Join(t.TempDir(), "repo"))
		Write(t, filepath.Join(root, "turbo.json"), `{"tasks":{"lint":{}}}`)
		path := filepath.Join(t.TempDir(), "classifier")
		Stub(t, path, stub)
		k.Overlay("classifier: [" + strconv.Quote(path) + "]\n")
		k.Dir = root
		return k, root
	}
	lint := sources.Entry{Source: "turbo", File: "turbo.json", Name: "lint", Dir: ".", Body: sources.Body{Text: "turbo run lint"}}

	t.Run("prints each entry's verified verdict", func(t *testing.T) {
		t.Parallel()
		answer := `{"is_error":false,"subtype":"success","structured_output":{"entries":[{"id":"` + gapfill.Key(lint) + `","role":"check","kind":"lint","mutates":false,"affected_form":"turbo run lint --filter=...[{base}]"}],"managers":[],"proposals":[]}}`
		k, _ := setup(t, "cat >/dev/null\necho '"+answer+"'\n")
		r := k.Run("", "enforce", "classify")
		r.Want(t, 0)
		r.Has(t, "turbo\tturbo.json\t-\t.\tlint\t-\tturbo run lint\tcheck:lint; affected: turbo run lint --filter=...[{base}]\n")
	})
	t.Run("a classifier that fails leaves entries unclassified, with its own exit code", func(t *testing.T) {
		t.Parallel()
		k, _ := setup(t, "cat >/dev/null\nexit 1\n")
		r := k.Run("", "enforce", "classify")
		r.Want(t, 125)
		r.Has(t, "turbo run lint\tunclassified (classifier: exit status 1)\n")
	})
	t.Run("run-checks on the plan exits with its failure count", func(t *testing.T) {
		t.Parallel()
		answer := `{"is_error":false,"subtype":"success","structured_output":{"entries":[{"id":"` + gapfill.Key(lint) + `","role":"check","kind":"lint","mutates":false}],"managers":[],"proposals":[]}}`
		k, root := setup(t, "cat >/dev/null\necho '"+answer+"'\n")
		Stub(t, filepath.Join(root, "node_modules", ".bin", "turbo"), "echo lint failed\nexit 1\n")
		Write(t, filepath.Join(root, ".gitignore"), "node_modules\n")
		r := k.Run("", "run-checks")
		r.Want(t, 1)
		r.Has(t, "FAIL lint (turbo.json: lint)\n  cmd: turbo run lint\n", "lint failed\n", "\nchecks: 0 passed, 1 failed, ")
	})
	t.Run("a gate that changes files is marked, and gates reset --restore undoes it", func(t *testing.T) {
		t.Parallel()
		answer := `{"is_error":false,"subtype":"success","structured_output":{"entries":[{"id":"` + gapfill.Key(lint) + `","role":"check","kind":"lint","mutates":false}],"managers":[],"proposals":[]}}`
		k, root := setup(t, "cat >/dev/null\necho '"+answer+"'\n")
		Stub(t, filepath.Join(root, "node_modules", ".bin", "turbo"), "echo fixed > a.ts\n")
		Write(t, filepath.Join(root, ".gitignore"), "node_modules\n")
		Write(t, filepath.Join(root, "a.ts"), "edited\n")
		r := k.Run("", "run-checks")
		r.Want(t, 1)
		r.Has(t, "FAIL lint (turbo.json: lint)\n", "  changed: a.ts; it won't run again until kit gates reset")
		k.Run("", "run-checks").Has(t, "SKIP lint (turbo.json: lint) (changed a.ts when it last ran; kit gates reset lets it run again)")
		r = k.Run("", "gates", "reset", "--restore")
		r.Want(t, 0)
		r.Has(t, "restored a.ts\n", "1 gate(s) will run again\n")
		if got := Read(t, filepath.Join(root, "a.ts")); got != "edited\n" {
			t.Errorf("a.ts = %q, want the edit back", got)
		}
		k.Run("", "run-checks").Has(t, "FAIL lint (turbo.json: lint)\n")
	})
	t.Run("run-checks on the plan exits apart when an entry is unclassified", func(t *testing.T) {
		t.Parallel()
		k, _ := setup(t, "cat >/dev/null\nexit 1\n")
		r := k.Run("", "run-checks")
		r.Want(t, 125)
		r.Has(t, "SKIP turbo.json: lint (unclassified (classifier: exit status 1))\n", "checks: 0 passed, 0 failed, ")
	})
	t.Run("under the classifier's guard, a hook does nothing", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		k.Setenv(gapfill.Guard, "1")
		r := k.Hook("guard-dispatch", Payload("Write", filepath.Join(k.Home, ".ssh", "id_rsa"), "s1", k.Home))
		r.Want(t, 0)
		r.Empty(t)
	})
}
