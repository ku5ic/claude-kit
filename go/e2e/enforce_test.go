package e2e

import (
	"path/filepath"
	"testing"
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
	t.Run("anything but --list is a usage error", func(t *testing.T) {
		t.Parallel()
		r := New(t).Run("", "enforce")
		r.Want(t, 2)
		r.Has(t, "usage: kit enforce --list")
	})
}
