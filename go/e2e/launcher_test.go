package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// bin/kit picks the binary for this platform and plugin version. These run
// the real launcher, so they need go/build.sh run first (CI builds before
// testing).
func TestLauncher(t *testing.T) {
	k := New(t)
	kitFile := func(name string) string { return filepath.Join(kitRoot, name) }
	t.Run("a hook blocks through the built binary", func(t *testing.T) {
		r := k.exec(kitFile("bin/kit"), `{"tool_name":"Bash","tool_input":{"command":"git push --force origin main"}}`, "hook", "guard-bash")
		r.Want(t, 2)
		r.Has(t, "Blocked by guard-bash:")
	})
	t.Run("a subcommand prints through the built binary", func(t *testing.T) {
		r := k.exec(kitFile("bin/kit"), "", "plans-dir")
		r.Want(t, 0)
		if r.Stdout == "" {
			t.Error("plans-dir printed nothing")
		}
	})

	// A copy of the launcher with no binary beside it.
	bare := filepath.Join(t.TempDir(), "kit")
	raw, err := os.ReadFile(kitFile("bin/kit"))
	if err != nil {
		t.Fatal(err)
	}
	Write(t, bare, string(raw))
	t.Run("without a binary a hook fails open", func(t *testing.T) {
		r := k.exec("bash", "{}", bare, "hook", "guard-bash")
		r.Want(t, 0)
		r.Has(t, "kit: no binary for")
	})
	t.Run("without a binary SessionStart tells the user and Claude in one JSON object", func(t *testing.T) {
		r := k.exec("bash", "{}", bare, "hook", "inject-context")
		r.Want(t, 0)
		var out struct {
			SystemMessage      string `json:"systemMessage"`
			HookSpecificOutput struct {
				HookEventName     string `json:"hookEventName"`
				AdditionalContext string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
			t.Fatalf("stdout is not one JSON object: %v\n%s", err, r.Stdout)
		}
		if !strings.Contains(out.SystemMessage, "guard is off") ||
			out.HookSpecificOutput.HookEventName != "SessionStart" ||
			out.HookSpecificOutput.AdditionalContext != out.SystemMessage {
			t.Errorf("unexpected output:\n%s", r.Stdout)
		}
	})
	t.Run("without a binary other hooks print nothing to stdout", func(t *testing.T) {
		r := k.exec("bash", "{}", bare, "hook", "guard-bash")
		r.Want(t, 0)
		if r.Stdout != "" {
			t.Errorf("stdout = %q", r.Stdout)
		}
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

	t.Run("a failed download is not retried within a minute", func(t *testing.T) {
		root := t.TempDir()
		Write(t, filepath.Join(root, "bin/kit"), string(raw))
		Write(t, filepath.Join(root, ".claude-plugin/plugin.json"), `{"name": "claude-kit", "version": "9.9.9"}`)
		stubs := t.TempDir()
		log := filepath.Join(stubs, "curl.log")
		Write(t, filepath.Join(stubs, "curl"), "#!/bin/sh\necho \"$@\" >>"+log+"\nexit 22\n")
		if err := os.Chmod(filepath.Join(stubs, "curl"), 0o755); err != nil {
			t.Fatal(err)
		}
		k := New(t)
		k.PrependPath(stubs)
		launcher := filepath.Join(root, "bin/kit")
		k.exec("bash", "{}", launcher, "hook", "inject-context").Want(t, 0)
		k.exec("bash", "{}", launcher, "hook", "inject-context").Want(t, 0)
		if n := strings.Count(Read(t, log), "\n"); n != 1 {
			t.Errorf("curl ran %d times, want once", n)
		}
	})

	t.Run("a guard never downloads, since its timeout is shorter than curl's", func(t *testing.T) {
		root := t.TempDir()
		Write(t, filepath.Join(root, "bin/kit"), string(raw))
		Write(t, filepath.Join(root, ".claude-plugin/plugin.json"), `{"name": "claude-kit", "version": "9.9.9"}`)
		stubs := t.TempDir()
		log := filepath.Join(stubs, "curl.log")
		Write(t, filepath.Join(stubs, "curl"), "#!/bin/sh\necho \"$@\" >>"+log+"\nexit 22\n")
		if err := os.Chmod(filepath.Join(stubs, "curl"), 0o755); err != nil {
			t.Fatal(err)
		}
		k := New(t)
		k.PrependPath(stubs)
		k.exec("bash", "{}", filepath.Join(root, "bin/kit"), "hook", "guard-bash").Want(t, 0)
		if _, err := os.Stat(log); err == nil {
			t.Errorf("guard-bash ran curl:\n%s", Read(t, log))
		}
	})
}
