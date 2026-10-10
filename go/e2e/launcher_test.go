package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// bin/kit picks the binary for this platform and plugin version. These run
// the real launcher, so they need go/build.sh run first (CI builds before
// testing).
func TestLauncher(t *testing.T) {
	t.Parallel()
	k := New(t)
	kitFile := func(name string) string { return filepath.Join(kitRoot, name) }
	t.Run("a hook blocks through the built binary", func(t *testing.T) {
		r := k.exec(kitFile("bin/kit"), `{"tool_name":"Bash","tool_input":{"command":"git push --force origin main"}}`, "hook", "guard-bash")
		r.Want(t, 2)
		r.Has(t, "Blocked by guard-bash:")
	})
	t.Run("a subcommand prints through the built binary", func(t *testing.T) {
		r := k.exec(kitFile("bin/kit"), "", "version")
		r.Want(t, 0)
		if r.Stdout == "" {
			t.Error("version printed nothing")
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
		r := k.exec("bash", "", bare, "version")
		r.Want(t, 127)
		r.Has(t, "build it with go/build.sh")
	})

	// install is a plugin tree whose plugin.json names version 9.9.9, with
	// no binary yet, and a sandbox whose curl records each run in log, then
	// runs extra.
	install := func(t *testing.T, extra string) (k *Kit, launcher, log string) {
		root := t.TempDir()
		Write(t, filepath.Join(root, "bin/kit"), string(raw))
		Write(t, filepath.Join(root, ".claude-plugin/plugin.json"), `{"name": "claude-kit", "version": "9.9.9"}`)
		stubs := t.TempDir()
		log = filepath.Join(stubs, "curl.log")
		testutil.FakeTool(t, filepath.Join(stubs, "curl"), log, extra)
		k = New(t)
		k.PrependPath(stubs)
		return k, filepath.Join(root, "bin/kit"), log
	}

	t.Run("a missing binary is downloaded from the matching release once", func(t *testing.T) {
		k, launcher, log := install(t, `while [ "$1" != -o ]; do shift; done
printf '#!/bin/sh\necho fetched "$@"\n' >"$2"`)
		r := k.exec("bash", "", launcher, "version")
		r.Want(t, 0)
		r.Has(t, "fetched version")
		k.exec("bash", "", launcher, "version").Has(t, "fetched version")
		calls := testutil.Calls(t, log)
		if len(calls) != 1 {
			t.Errorf("curl ran %d times, want once:\n%s", len(calls), strings.Join(calls, "\n"))
		}
		want := "releases/download/v9.9.9/kit-9.9.9-" + runtime.GOOS + "-" + runtime.GOARCH
		if !strings.Contains(strings.Join(calls, "\n"), want) {
			t.Errorf("curl URL lacks %s:\n%s", want, strings.Join(calls, "\n"))
		}
	})

	t.Run("KIT_DEV builds into bin/ when the launcher is called by a relative path", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		Write(t, filepath.Join(root, "bin/kit"), string(raw))
		Write(t, filepath.Join(root, ".claude-plugin/plugin.json"), `{"name": "claude-kit", "version": "9.9.9"}`)
		Write(t, filepath.Join(root, "go/cmd/kit/main.go"), "package main\n")
		stubs := t.TempDir()
		// go build -o <out>: writes a fake binary at out, as go does.
		Stub(t, filepath.Join(stubs, "go"), `while [ "$1" != -o ]; do shift; done
mkdir -p "$(dirname "$2")"
printf '#!/bin/sh\necho built "$@"\n' >"$2"
chmod +x "$2"
`)
		k := New(t)
		k.Dir = root
		k.PrependPath(stubs)
		k.Setenv("KIT_DEV", "1")
		r := k.exec("bash", "", "bin/kit", "x")
		r.Want(t, 0)
		r.Has(t, "built x")
		if Exists(filepath.Join(root, "go/bin")) {
			t.Error("the build wrote under go/bin")
		}
	})

	t.Run("a failed download is not retried within a minute", func(t *testing.T) {
		k, launcher, log := install(t, "exit 22")
		k.exec("bash", "{}", launcher, "hook", "inject-context").Want(t, 0)
		k.exec("bash", "{}", launcher, "hook", "inject-context").Want(t, 0)
		if n := len(testutil.Calls(t, log)); n != 1 {
			t.Errorf("curl ran %d times, want once", n)
		}
	})

	t.Run("a guard never downloads, since its timeout is shorter than curl's", func(t *testing.T) {
		k, launcher, log := install(t, "exit 22")
		k.exec("bash", "{}", launcher, "hook", "guard-bash").Want(t, 0)
		if calls := testutil.Calls(t, log); calls != nil {
			t.Errorf("guard-bash ran curl:\n%s", strings.Join(calls, "\n"))
		}
	})

	t.Run("a status line never downloads, since each refresh cancels it", func(t *testing.T) {
		k, launcher, log := install(t, "exit 22")
		for _, cmd := range []string{"statusline", "subagent-statusline"} {
			k.exec("bash", "{}", launcher, cmd).Want(t, 127)
		}
		if calls := testutil.Calls(t, log); calls != nil {
			t.Errorf("a status line ran curl:\n%s", strings.Join(calls, "\n"))
		}
	})
}
