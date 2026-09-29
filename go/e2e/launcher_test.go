package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

// The shims source bin/kit, which picks the committed binary for this
// platform. These run the real shims, so they test the committed binary.
func TestLauncher(t *testing.T) {
	k := New(t)
	shim := func(name string) string { return filepath.Join(kitRoot, name) }
	t.Run("a hook shim blocks through the committed binary", func(t *testing.T) {
		r := k.exec(shim("hooks/guard-bash.sh"), `{"tool_name":"Bash","tool_input":{"command":"git push --force origin main"}}`)
		r.Want(t, 2)
		r.Has(t, "Blocked by guard-bash.sh")
	})
	t.Run("a bin shim prints through the committed binary", func(t *testing.T) {
		r := k.exec(shim("bin/plans-dir.sh"), "")
		r.Want(t, 0)
		if r.Stdout == "" {
			t.Error("plans-dir printed nothing")
		}
	})

	// A copy of the launcher with no binary beside it.
	bare := filepath.Join(t.TempDir(), "kit")
	raw, err := os.ReadFile(shim("bin/kit"))
	if err != nil {
		t.Fatal(err)
	}
	Write(t, bare, string(raw))
	t.Run("without a binary a hook fails open", func(t *testing.T) {
		r := k.exec("bash", "{}", bare, "hook", "guard-bash")
		r.Want(t, 0)
		r.Has(t, "kit: no binary for")
	})
	t.Run("without a binary a command fails with 127", func(t *testing.T) {
		r := k.exec("bash", "", bare, "plans-dir")
		r.Want(t, 127)
		r.Has(t, "build it with claude-kit/go/build.sh")
	})
}
