package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestMain(m *testing.M) {
	// Cases build their own repos; git must see only those, never the
	// developer's config, global ignore file, or a GIT_DIR inherited from a
	// git hook.
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); strings.HasPrefix(name, "GIT_") {
			os.Unsetenv(name)
		}
	}
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	os.Setenv("XDG_CONFIG_HOME", os.DevNull)
	os.Exit(m.Run())
}

const (
	version = "1.2.3"
	exe     = "/plugin/bin/kit-1.2.3"
)

// install is a healthy install and project, for a case to break.
type install struct {
	paths config.Paths
	repo  string
}

func newInstall(t *testing.T) install {
	t.Helper()
	dir := testutil.Physical(t, t.TempDir())
	root, claude, repo := filepath.Join(dir, "plugin"), filepath.Join(dir, "claude"), filepath.Join(dir, "repo")
	testutil.Put(t, root, "kit.yml", "tools: [git]\n")
	testutil.Put(t, root, ".claude-plugin/plugin.json", `{"version": "`+version+`"}`)
	testutil.Put(t, claude, "settings.json", `{"attribution": false, "sandbox": {"enabled": true}}`)
	testutil.Put(t, repo, ".gitignore", ".claude/scratch/\n")
	testutil.Git(t, repo, "init", "-q")
	return install{repo: repo, paths: config.Paths{
		Root:      root,
		ClaudeDir: claude,
		Base:      filepath.Join(root, "kit.yml"),
		Overlay:   filepath.Join(claude, "claude-kit.local.yml"),
		Settings:  filepath.Join(claude, "settings.json"),
		Manifest:  filepath.Join(root, ".claude-plugin", "plugin.json"),
	}}
}

func (i install) user(t *testing.T, body string) {
	t.Helper()
	testutil.Write(t, i.paths.Settings, body)
}

func (i install) project(t *testing.T, name, body string) {
	t.Helper()
	testutil.Put(t, i.repo, filepath.Join(".claude", name), body)
}

// run is the exit code and each line by its check's name, spaces collapsed.
func (i install) run(t *testing.T) (code int, lines map[string]string) {
	t.Helper()
	var out, errOut strings.Builder
	code = Run(i.paths, version, exe, i.repo, nil, &out, &errOut)
	if errOut.Len() > 0 {
		t.Errorf("stderr: %s", &errOut)
	}
	lines = map[string]string{}
	for line := range strings.Lines(out.String()) {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			t.Fatalf("line %q", line)
		}
		lines[fields[1]] = strings.Join(fields, " ")
	}
	return code, lines
}

func TestDoctorLines(t *testing.T) {
	t.Parallel()
	const off = `"attribution": false`
	for _, tc := range []struct {
		name    string
		breakIt func(t *testing.T, i *install)
		check   string
		want    []string // the line's parts, its state first
	}{
		{"a binary of the plugin's version", nil, "binary", []string{"ok", version, exe}},
		{"a binary of another version", func(t *testing.T, i *install) {
			testutil.Write(t, i.paths.Manifest, `{"version": "1.3.0"}`)
		}, "binary", []string{"problem", version, "1.3.0"}},
		{"a plugin that states no version", func(t *testing.T, i *install) {
			testutil.Write(t, i.paths.Manifest, `{}`)
		}, "binary", []string{"problem", "states no version"}},
		{"no overlay", nil, "overlay", []string{"ok", "none"}},
		{"an overlay", func(t *testing.T, i *install) {
			testutil.Write(t, i.paths.Overlay, "tools: [sh]\n")
		}, "overlay", []string{"ok", "claude-kit.local.yml"}},
		{"an overlay with an unknown key", func(t *testing.T, i *install) {
			testutil.Write(t, i.paths.Overlay, "nonsense: 1\n")
		}, "overlay", []string{"problem", "claude-kit.local.yml", "nonsense"}},
		{"a kit.yml that can't be read", func(t *testing.T, i *install) {
			if err := os.Remove(i.paths.Base); err != nil {
				t.Fatal(err)
			}
		}, "overlay", []string{"problem", "kit.yml"}},
		{"the sandbox on for the user", nil, "sandbox", []string{"info", "on", "settings.json"}},
		{"the sandbox set nowhere", func(t *testing.T, i *install) {
			i.user(t, "{"+off+"}")
		}, "sandbox", []string{"info", "off (default)"}},
		{"no user settings file", func(t *testing.T, i *install) {
			i.project(t, "settings.json", "{"+off+"}")
			if err := os.Remove(i.paths.Settings); err != nil {
				t.Fatal(err)
			}
		}, "sandbox", []string{"info", "off (default)"}},
		{"the sandbox on for the project", func(t *testing.T, i *install) {
			i.user(t, "{"+off+"}")
			i.project(t, "settings.json", `{"sandbox": {"enabled": true}}`)
		}, "sandbox", []string{"info", "on", "repo/.claude/settings.json"}},
		{"a project false over the user's true", func(t *testing.T, i *install) {
			i.project(t, "settings.json", `{"sandbox": {"enabled": false}}`)
		}, "sandbox", []string{"info", "off", "repo/.claude/settings.json"}},
		{"a local true over the project's false", func(t *testing.T, i *install) {
			i.project(t, "settings.json", `{"sandbox": {"enabled": false}}`)
			i.project(t, "settings.local.json", `{"sandbox": {"enabled": true}}`)
		}, "sandbox", []string{"info", "on", "settings.local.json"}},
		{"the user's false over a project's true", func(t *testing.T, i *install) {
			i.user(t, "{"+off+`, "sandbox": {"enabled": false}}`)
			i.project(t, "settings.local.json", `{"sandbox": {"enabled": true}}`)
		}, "sandbox", []string{"info", "off", "claude/settings.json"}},
		{"a sandbox key that leaves enabled out", func(t *testing.T, i *install) {
			i.project(t, "settings.local.json", `{"sandbox": {"failIfUnavailable": true}}`)
		}, "sandbox", []string{"info", "on", "claude/settings.json"}},
		{"attribution off", nil, "attribution", []string{"ok", "off"}},
		{"attribution set nowhere", func(t *testing.T, i *install) {
			i.user(t, `{"sandbox": {"enabled": true}}`)
		}, "attribution", []string{"problem", "on (default)"}},
		{"attribution on for the project", func(t *testing.T, i *install) {
			i.project(t, "settings.json", `{"attribution": {"pr": ""}}`)
		}, "attribution", []string{"problem", "on", "repo/.claude/settings.json"}},
		{"every tool on PATH", nil, "tools", []string{"ok", "none missing"}},
		{"a missing tool", func(t *testing.T, i *install) {
			testutil.Write(t, i.paths.Base, "tools: [git, no-such-tool-a, no-such-tool-b]\n")
		}, "tools", []string{"problem", "missing: no-such-tool-a, no-such-tool-b"}},
		{"scratch gitignored", nil, "scratch", []string{"ok", "gitignored", "repo/.claude/scratch"}},
		{"scratch not gitignored", func(t *testing.T, i *install) {
			testutil.Put(t, i.repo, ".gitignore", "")
		}, "scratch", []string{"problem", "not gitignored", "repo/.claude/scratch"}},
		{"outside a git repo", func(t *testing.T, i *install) {
			i.repo = t.TempDir()
		}, "scratch", []string{"ok", "outside a git repo"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			i := newInstall(t)
			if tc.breakIt != nil {
				tc.breakIt(t, &i)
			}
			code, lines := i.run(t)
			line := lines[tc.check]
			if !strings.HasPrefix(line, tc.want[0]+" ") {
				t.Errorf("%s line %q, want it %s", tc.check, line, tc.want[0])
			}
			testutil.Has(t, line, tc.want[1:]...)
			if want := map[string]int{"ok": 0, "info": 0, "problem": 1}[tc.want[0]]; code != want {
				t.Errorf("exit %d, want %d; lines %v", code, want, lines)
			}
		})
	}
}

// A kit.yml that can't be read names no tools, so none can be missing.
func TestDoctorSkipsToolsWithoutAConfig(t *testing.T) {
	t.Parallel()
	i := newInstall(t)
	if err := os.Remove(i.paths.Base); err != nil {
		t.Fatal(err)
	}
	if _, lines := i.run(t); lines["tools"] != "" || lines["scratch"] == "" {
		t.Errorf("lines %v", lines)
	}
}

func TestDoctorTakesNoArguments(t *testing.T) {
	t.Parallel()
	i := newInstall(t)
	var out, errOut strings.Builder
	if code := Run(i.paths, version, exe, i.repo, []string{"x"}, &out, &errOut); code != 2 || out.Len() > 0 || !strings.HasPrefix(errOut.String(), "usage: kit doctor") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, &out, &errOut)
	}
}

// doctor reads: nothing under the project changes, scratch included.
func TestDoctorLeavesTheProjectAlone(t *testing.T) {
	t.Parallel()
	i := newInstall(t)
	i.run(t)
	if _, err := os.Stat(filepath.Join(i.repo, ".claude")); !os.IsNotExist(err) {
		t.Errorf("doctor made %s/.claude: %v", i.repo, err)
	}
}
