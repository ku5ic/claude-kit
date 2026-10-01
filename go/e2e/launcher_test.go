package e2e

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The shims source bin/kit, which picks the binary for this platform and
// plugin version. These run the real shims, so they need go/build.sh run
// first (CI builds before testing).
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
		r.Has(t, "build it with go/build.sh")
	})

	// An install: plugin.json names the version, no binary yet, and a stub
	// curl that records its URL and writes a fake binary.
	t.Run("a missing binary is downloaded from the matching release once", func(t *testing.T) {
		root := t.TempDir()
		Write(t, filepath.Join(root, "bin/kit"), string(raw))
		Write(t, filepath.Join(root, ".claude-plugin/plugin.json"), `{"name": "claude-kit", "version": "9.9.9"}`)
		stubs := t.TempDir()
		log := filepath.Join(stubs, "curl.log")
		Write(t, filepath.Join(stubs, "curl"), `#!/bin/sh
echo "$@" >>`+log+`
while [ "$1" != -o ]; do shift; done
printf '#!/bin/sh\necho fetched "$@"\n' >"$2"
`)
		if err := os.Chmod(filepath.Join(stubs, "curl"), 0o755); err != nil {
			t.Fatal(err)
		}
		k := New(t)
		k.PrependPath(stubs)
		launcher := filepath.Join(root, "bin/kit")
		r := k.exec("bash", "", launcher, "plans-dir")
		r.Want(t, 0)
		r.Has(t, "fetched plans-dir")
		k.exec("bash", "", launcher, "plans-dir").Has(t, "fetched plans-dir")
		calls := Read(t, log)
		if strings.Count(calls, "\n") != 1 {
			t.Errorf("curl ran %d times, want once:\n%s", strings.Count(calls, "\n"), calls)
		}
		want := "releases/download/v9.9.9/kit-9.9.9-" + runtime.GOOS + "-" + runtime.GOARCH
		if !strings.Contains(calls, want) {
			t.Errorf("curl URL lacks %s:\n%s", want, calls)
		}
	})
}
