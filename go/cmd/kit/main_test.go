package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestParseRunChecksArgs(t *testing.T) {
	t.Parallel()
	a, err := parseRunChecksArgs([]string{"--plan", "--only", "./services/api/", "--only", "packages/a/"})
	if err != nil || !a.plan || !slices.Equal(a.only, []string{"services/api", "packages/a"}) {
		t.Errorf("--plan with --only repeated, paths cleaned: %+v, %v", a, err)
	}
	for want, args := range map[string][]string{
		`unknown argument "--plann"`:     {"--plann"},
		"--only needs at least one":      {"--only"},
		"--plan must come before --only": {"--only", ".", "--plan"},
		`unknown argument "-x"`:          {"--only", ".", "-x"},
		`unknown argument "extra"`:       {"--plan", "extra"},
	} {
		if _, err := parseRunChecksArgs(args); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", args, err, want)
		}
	}
}

// tmp is a physical temp dir, as git rev-parse reports paths.
func tmp(t *testing.T) string {
	t.Helper()
	return testutil.Physical(t, t.TempDir())
}

func mkdir(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func gitInit(t *testing.T, dir string) string {
	t.Helper()
	testutil.Git(t, mkdir(t, dir), "init", "-q", "-b", "main")
	return dir
}

// call runs one command's func from cwd with a throwaway kit home.
func call(t *testing.T, cmd func(*env, []string) int, cwd string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut strings.Builder
	e := &env{paths: config.Paths{Home: filepath.Join(tmp(t), ".claude")}, stdout: &out, stderr: &errOut, cwd: cwd}
	code = cmd(e, args)
	return code, out.String(), errOut.String()
}

// sandbox points run at the real kit.yml and a throwaway HOME and
// CLAUDE_CONFIG_DIR. It sets env, so its tests can't be parallel.
func sandbox(t *testing.T) (home, configDir string) {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := tmp(t)
	home, configDir = filepath.Join(dir, "home"), filepath.Join(dir, "config")
	t.Setenv("CLAUDE_PLUGIN_ROOT", root)
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	t.Setenv("HOME", home)
	return home, configDir
}

// runFrom is run with dir as the cwd and $PWD, as a shell's cd leaves them.
func runFrom(t *testing.T, dir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	t.Chdir(dir)
	var out, errOut strings.Builder
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestScratchDirKindAndSlugPrintAReportPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		file string // the report's name before its timestamp
	}{
		{"<kind> <slug> name the report", []string{"perf", "checkout-page"}, "perf-checkout-page"},
		{"<kind> alone leaves the slug out", []string{"deps"}, "deps"},
		{"the slug is made filename-safe", []string{"review", "feat/login page"}, "review-feat-login-page"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := gitInit(t, filepath.Join(tmp(t), "repo"))
			code, stdout, stderr := call(t, cmdScratchDir, repo, tt.args...)
			pattern := `^` + regexp.QuoteMeta(filepath.Join(repo, ".claude/scratch", tt.file)) + `-[0-9]{8}-[0-9]{4}\.md\n$`
			if code != 0 || !regexp.MustCompile(pattern).MatchString(stdout) {
				t.Errorf("exit %d, stdout %q, want %s; stderr %q", code, stdout, pattern, stderr)
			}
		})
	}
}

func TestProjectRootCheckExitsByAnchoringAndPrintsNothing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string) (cwd string)
		code  int
	}{
		{
			name: "inside a repo exits 0",
			setup: func(t *testing.T, dir string) string {
				return mkdir(t, filepath.Join(gitInit(t, filepath.Join(dir, "repo")), "src"))
			},
		},
		{
			name: "outside a repo, with a language manifest two levels up, exits 0",
			setup: func(t *testing.T, dir string) string {
				testutil.Put(t, dir, "proj/package.json", "")
				return mkdir(t, filepath.Join(dir, "proj/a/b"))
			},
		},
		{
			name:  "outside a repo with no manifest exits 1",
			setup: func(t *testing.T, dir string) string { return mkdir(t, filepath.Join(dir, "plain/a/b")) },
			code:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := call(t, cmdProjectRoot, tt.setup(t, tmp(t)), "--check")
			if code != tt.code || stdout != "" || stderr != "" {
				t.Errorf("exit %d, want %d; stdout %q, stderr %q", code, tt.code, stdout, stderr)
			}
		})
	}
}

// The kit reads its cwd from $PWD, so an unanchored root is the logical
// path, not the physical one (macOS: /var -> /private/var).
func TestProjectRootUnanchoredPrintsTheLogicalPWD(t *testing.T) {
	sandbox(t)
	dir := mkdir(t, filepath.Join(t.TempDir(), "plain/a/b"))
	code, stdout, stderr := runFrom(t, dir, "project-root")
	if code != 0 || stdout != dir+"\n" {
		t.Errorf("exit %d, stdout %q, want %q; stderr %q", code, stdout, dir, stderr)
	}
}

// Help, version, and completion need no paths, so they answer where none
// resolve.
func TestHelpAndVersionNeedNoHome(t *testing.T) {
	sandbox(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("HOME", "")
	for _, c := range []struct{ arg, want string }{
		{"version", version + "\n"},
		{"--version", version + "\n"},
		{"--help", "git-base"},
		{"help", "the base ref of this checkout"},
		{"completion bash", "-F _kit kit"},
	} {
		code, stdout, stderr := runFrom(t, tmp(t), strings.Fields(c.arg)...)
		if code != 0 || !strings.Contains(stdout, c.want) {
			t.Errorf("kit %s: exit %d, stdout %q lacks %q; stderr %q", c.arg, code, stdout, c.want, stderr)
		}
	}
}

func TestScratchDirFollowsClaudeConfigDir(t *testing.T) {
	home, configDir := sandbox(t)
	outside := mkdir(t, filepath.Join(tmp(t), "plain/a/b"))
	if _, stdout, _ := runFrom(t, outside, "scratch-dir"); stdout != filepath.Join(configDir, "scratch")+"\n" {
		t.Errorf("fallback %q, want under %s", stdout, configDir)
	}

	repo := gitInit(t, filepath.Join(tmp(t), "repo"))
	runFrom(t, repo, "scratch-dir")
	got, err := os.ReadFile(filepath.Join(configDir, "logs/scratch-registry.txt"))
	if want := filepath.Join(repo, ".claude/scratch") + "\n"; err != nil || string(got) != want {
		t.Errorf("registry %q, %v; want %q", got, err, want)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude/logs/scratch-registry.txt")); err == nil {
		t.Error("registered under HOME despite CLAUDE_CONFIG_DIR")
	}
}
