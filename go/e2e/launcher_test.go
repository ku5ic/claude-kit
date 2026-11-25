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
		r.Has(t, "kit bash: BLOCKED ")
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
	t.Run("without a binary a hook fails open, printing nothing to stdout", func(t *testing.T) {
		r := k.exec("bash", "{}", bare, "hook", "guard-bash")
		r.Want(t, 0)
		r.Has(t, "kit: no binary for")
		if r.Stdout != "" {
			t.Errorf("stdout = %q", r.Stdout)
		}
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

	// release is a curl that serves a fake binary, and as its .sha256 the
	// checksum of sum's output.
	release := func(sum string) string {
		return `bin='#!/bin/sh\necho fetched "$@"\n'
case "$*" in *.sha256*) ` + sum + ` | { sha256sum 2>/dev/null || shasum -a 256; }; exit ;; esac
while [ "$1" != -o ]; do shift; done
printf "$bin" >"$2"`
	}

	t.Run("a missing binary is downloaded from the matching release once, checksum first", func(t *testing.T) {
		k, launcher, log := install(t, release(`printf "$bin"`))
		r := k.exec("bash", "", launcher, "version")
		r.Want(t, 0)
		r.Has(t, "fetched version")
		k.exec("bash", "", launcher, "version").Has(t, "fetched version")
		calls := testutil.Calls(t, log)
		if len(calls) != 2 {
			t.Errorf("curl ran %d times, want twice, the .sha256 and the binary:\n%s", len(calls), strings.Join(calls, "\n"))
		}
		want := "releases/download/v9.9.9/kit-9.9.9-" + runtime.GOOS + "-" + runtime.GOARCH
		if !strings.Contains(strings.Join(calls, "\n"), want+".sha256") {
			t.Errorf("curl URL lacks %s.sha256:\n%s", want, strings.Join(calls, "\n"))
		}
	})

	t.Run("a download whose checksum doesn't match never runs", func(t *testing.T) {
		k, launcher, _ := install(t, release(`printf 'something else'`))
		r := k.exec("bash", "", launcher, "version")
		r.Want(t, 127)
		r.Has(t, "no binary for")
	})

	t.Run("KIT_DEV builds into bin/ when the launcher is called by a relative path", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		Write(t, filepath.Join(root, "bin/kit"), string(raw))
		Write(t, filepath.Join(root, ".claude-plugin/plugin.json"), `{"name": "claude-kit", "version": "9.9.9"}`)
		Write(t, filepath.Join(root, "go/cmd/kit/main.go"), "package main\n")
		build, err := os.ReadFile(kitFile("go/build.sh"))
		if err != nil {
			t.Fatal(err)
		}
		Stub(t, filepath.Join(root, "go/build.sh"), string(build))
		stubs := t.TempDir()
		// go env answers build.sh's host target; go build -o <out> writes a
		// fake binary at out, as go does.
		Stub(t, filepath.Join(stubs, "go"), `[ "$1" = env ] && exit 0
while [ "$1" != -o ]; do shift; done
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
		k.exec("bash", "{}", launcher, "statusline").Want(t, 127)
		if calls := testutil.Calls(t, log); calls != nil {
			t.Errorf("a status line ran curl:\n%s", strings.Join(calls, "\n"))
		}
	})

	// marketplace is install in Claude Code's layout: the launcher under
	// marketplaces/ku5ic/bin, with nothing beside it, and a fake binary in
	// cache/ku5ic/kit/<version>/bin for each of versions.
	marketplace := func(t *testing.T, extra string, versions ...string) (k *Kit, launcher, log string) {
		k, from, log := install(t, extra)
		plugins := filepath.Join(t.TempDir(), "plugins")
		root := filepath.Join(plugins, "marketplaces/ku5ic")
		testutil.Mkdir(t, filepath.Dir(root))
		if err := os.Rename(filepath.Dir(filepath.Dir(from)), root); err != nil {
			t.Fatal(err)
		}
		for _, v := range versions {
			name := "kit-" + v + "-" + runtime.GOOS + "-" + runtime.GOARCH
			Stub(t, filepath.Join(plugins, "cache/ku5ic/kit", v, "bin", name), "echo cached "+v+" \"$@\"\n")
		}
		return k, filepath.Join(root, "bin/kit"), log
	}
	noCurl := func(t *testing.T, log string) {
		t.Helper()
		if calls := testutil.Calls(t, log); calls != nil {
			t.Errorf("curl ran:\n%s", strings.Join(calls, "\n"))
		}
	}

	t.Run("a marketplace launcher with no binary runs the same version's from the cache, with no download", func(t *testing.T) {
		k, launcher, log := marketplace(t, "exit 22", "9.9.8", "9.9.9")
		for _, args := range [][]string{{"statusline"}, {"hook", "guard-bash"}, {"version"}, {"hook", "inject-context"}} {
			r := k.exec("bash", "{}", append([]string{launcher}, args...)...)
			r.Want(t, 0)
			if got, want := r.Stdout, "cached 9.9.9 "+strings.Join(args, " ")+"\n"; got != want {
				t.Errorf("%v: stdout = %q, want %q", args, got, want)
			}
		}
		noCurl(t, log)
	})

	t.Run("a marketplace launcher never runs another version's", func(t *testing.T) {
		k, launcher, log := marketplace(t, release(`printf "$bin"`), "9.9.8")
		r := k.exec("bash", "", launcher, "statusline")
		r.Want(t, 127)
		if r.Stdout != "" {
			t.Errorf("a status line printed %q", r.Stdout)
		}
		r = k.exec("bash", "{}", launcher, "hook", "guard-bash")
		r.Want(t, 0)
		if r.Stdout != "" {
			t.Errorf("a guard printed %q", r.Stdout)
		}
		noCurl(t, log)
		k.exec("bash", "", launcher, "version").Has(t, "fetched version")
	})

	t.Run("a marketplace launcher with no cache downloads for the CLI alone", func(t *testing.T) {
		k, launcher, log := marketplace(t, release(`printf "$bin"`))
		k.exec("bash", "", launcher, "statusline").Want(t, 127)
		noCurl(t, log)
		k.exec("bash", "", launcher, "version").Has(t, "fetched version")
	})

	t.Run("its own binary comes before the cache's", func(t *testing.T) {
		k, launcher, _ := marketplace(t, "exit 22", "9.9.9")
		Stub(t, filepath.Join(filepath.Dir(launcher), "kit-9.9.9-"+runtime.GOOS+"-"+runtime.GOARCH), "echo own\n")
		k.exec("bash", "", launcher, "version").Has(t, "own")
	})
}
