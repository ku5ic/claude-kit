package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var (
	kitBin  string // the binary built from this tree, once per run
	kitRoot string // claude-kit/, the real plugin root
	// kitBinName is the file bin/kit runs: kit-<plugin.json version>-<os>-<arch>.
	kitBinName string
	// fakes is first on every run's PATH: its claude fails, so no run can
	// reach the real classifier.
	fakes string
	// homes holds every sandbox's HOME, removed with the run rather than
	// with each test: the gap-fill run SessionStart starts in the
	// background can still be writing to its cache when the test ends.
	homes string
)

func TestMain(m *testing.M) {
	os.Exit(func() int {
		dir, err := os.MkdirTemp("", "kit-e2e")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer os.RemoveAll(dir)
		kitBin = filepath.Join(dir, "kit")
		if out, err := exec.Command("go", "build", "-o", kitBin, "../cmd/kit").CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "build kit: %v\n%s", err, out)
			return 1
		}
		fakes, homes = filepath.Join(dir, "fakes"), filepath.Join(dir, "homes")
		err = os.MkdirAll(homes, 0o755)
		if err == nil {
			err = os.MkdirAll(fakes, 0o755)
		}
		if err == nil {
			err = os.WriteFile(filepath.Join(fakes, "claude"), []byte("#!/bin/sh\nexit 1\n"), 0o755)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		root, err := filepath.Abs("../..")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		kitRoot = root
		raw, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "plugin.json"))
		var manifest struct{ Version string }
		if err == nil {
			err = json.Unmarshal(raw, &manifest)
		}
		if err != nil || manifest.Version == "" {
			fmt.Fprintln(os.Stderr, "plugin.json version:", err)
			return 1
		}
		kitBinName = "kit-" + manifest.Version + "-" + runtime.GOOS + "-" + runtime.GOARCH
		return m.Run()
	}())
}

// Kit is one test's sandbox: a fake HOME with .claude/logs, the plugin
// root the kit reads kit.yml from, and the cwd and extra environment runs
// get.
type Kit struct {
	t      *testing.T
	Home   string // $HOME
	Claude string // $HOME/.claude
	Root   string // CLAUDE_PLUGIN_ROOT
	Dir    string // cwd of each run; Home by default
	Env    []string
}

// New is a sandbox reading the real kit.yml.
func New(t *testing.T) *Kit {
	t.Helper()
	dir, err := os.MkdirTemp(homes, "home")
	if err != nil {
		t.Fatal(err)
	}
	home := Physical(t, dir)
	k := &Kit{t: t, Home: home, Claude: filepath.Join(home, ".claude"), Root: kitRoot, Dir: home}
	Mkdir(t, filepath.Join(k.Claude, "logs"))
	return k
}

// NewPlugin is a sandbox whose plugin root is the fake .claude, so the kit
// reads the kit.yml a test writes there.
func NewPlugin(t *testing.T) *Kit {
	k := New(t)
	k.Root = k.Claude
	return k
}

// KitYML writes the plugin root's kit.yml (NewPlugin sandboxes only).
func (k *Kit) KitYML(body string) {
	k.t.Helper()
	if k.Root == kitRoot {
		k.t.Fatal("KitYML on a sandbox reading the real kit.yml")
	}
	Write(k.t, filepath.Join(k.Root, "kit.yml"), body)
}

// Overlay writes the user overlay, claude-kit.local.yml.
func (k *Kit) Overlay(body string) {
	k.t.Helper()
	Write(k.t, filepath.Join(k.Claude, "claude-kit.local.yml"), body)
}

// Result is one run: its exit status, and its output apart and merged in
// the order it was written (bats' $output).
type Result struct {
	Status                 int
	Stdout, Stderr, Output string
}

// Run runs `kit <args>` with stdin.
func (k *Kit) Run(stdin string, args ...string) Result {
	k.t.Helper()
	return k.exec(kitBin, stdin, args...)
}

// Shell runs a bash script in the sandbox's environment, for a test that
// drives the launcher or composes commands; bash -c, with $KIT set to the
// freshly built binary.
func (k *Kit) Shell(stdin, script string) Result {
	k.t.Helper()
	return k.exec("bash", stdin, "-c", script)
}

// Hook runs `kit hook <name>` with payload: a string as is, anything else
// as JSON.
func (k *Kit) Hook(name string, payload any) Result {
	k.t.Helper()
	stdin, ok := payload.(string)
	if !ok {
		raw, err := json.Marshal(payload)
		if err != nil {
			k.t.Fatal(err)
		}
		stdin = string(raw)
	}
	return k.Run(stdin, "hook", name)
}

func (k *Kit) exec(name, stdin string, args ...string) Result {
	k.t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = k.Dir
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Env = k.environ()
	var stdout, stderr bytes.Buffer
	merged := &lockedBuffer{}
	cmd.Stdout = &tee{&stdout, merged}
	cmd.Stderr = &tee{&stderr, merged}
	err := cmd.Run()
	var exit *exec.ExitError
	status := 0
	switch {
	case errors.As(err, &exit):
		status = exit.ExitCode()
	case err != nil:
		k.t.Fatalf("run %s %q: %v", name, args, err)
	}
	return Result{status, stdout.String(), stderr.String(), merged.String()}
}

// environ is the test process's environment minus anything that could
// point the kit at the developer's real config or tool environments, plus
// the sandbox's.
func (k *Kit) environ() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "CLAUDE") || strings.HasPrefix(name, "KIT_") || strings.HasPrefix(name, "GIT_") ||
			strings.HasPrefix(name, "ASDF_") || strings.HasPrefix(name, "MISE_") || name == "VIRTUAL_ENV" || name == "CONDA_PREFIX" {
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"PATH="+basePath(),
		"HOME="+k.Home,
		"PWD="+k.Dir, // the kit reads its cwd from PWD, as a shell's cd sets it
		"CLAUDE_PLUGIN_ROOT="+k.Root,
		"KIT="+kitBin,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	return append(env, k.Env...)
}

// basePath is every run's PATH before PrependPath: the fakes, then the
// test process's own.
func basePath() string { return fakes + ":" + os.Getenv("PATH") }

// Setenv adds KEY=value to every later run.
func (k *Kit) Setenv(key, value string) { k.Env = append(k.Env, key+"="+value) }

// PrependPath puts dir first on PATH for every later run, ahead of any
// dir an earlier call prepended.
func (k *Kit) PrependPath(dir string) {
	path := basePath()
	for _, kv := range k.Env {
		if v, ok := strings.CutPrefix(kv, "PATH="); ok {
			path = v
		}
	}
	k.Setenv("PATH", dir+":"+path)
}

// Git runs git in dir with the sandbox's environment and returns its
// trimmed output; a failure fails the test.
func (k *Kit) Git(dir string, args ...string) string {
	k.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = k.environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		k.t.Fatalf("git %q: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// Repo makes dir (created if needed) a git repository with one empty
// commit, and returns its physical path.
func (k *Kit) Repo(dir string) string {
	k.t.Helper()
	Mkdir(k.t, dir)
	dir = Physical(k.t, dir)
	k.Git(dir, "init", "-q")
	k.Git(dir, "commit", "-q", "--allow-empty", "-m", "init")
	return dir
}

// Tree is a kit tree for the freshly built binary, for tests that run the
// real bin/kit launcher: bin/kit and the plugin.json it reads the version
// from, copied from the kit, the binary as bin/<kitBinName>, and rules
// linked to the kit's. The binary is a hard link or a copy, never a
// symlink: the kit resolves symlinks to find its root, and reads the rules
// it injects from there.
func Tree(t *testing.T) string {
	t.Helper()
	dir := Physical(t, t.TempDir())
	for _, src := range []string{"bin/kit", ".claude-plugin/plugin.json"} {
		dst := filepath.Join(dir, src)
		Write(t, dst, Read(t, filepath.Join(kitRoot, src)))
		if err := os.Chmod(dst, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(dir, "bin", kitBinName)
	if err := os.Link(kitBin, bin); err != nil {
		Write(t, bin, Read(t, kitBin))
		if err := os.Chmod(bin, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(kitRoot, "rules"), filepath.Join(dir, "rules")); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Payload is a PreToolUse payload: tool_input.command for Bash,
// tool_input.file_path for any other tool; an empty session or cwd is
// left out.
func Payload(tool, target, session, cwd string) map[string]any {
	input := map[string]any{"file_path": target}
	if tool == "Bash" {
		input = map[string]any{"command": target}
	}
	p := map[string]any{"tool_name": tool, "tool_input": input}
	if session != "" {
		p["session_id"] = session
	}
	if cwd != "" {
		p["cwd"] = cwd
	}
	return p
}

// Write writes body to path, creating its directories.
func Write(t *testing.T, path, body string) {
	t.Helper()
	Mkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Stub writes an executable bash script to path; body follows the shebang.
func Stub(t *testing.T, path, body string) {
	t.Helper()
	Write(t, path, "#!/usr/bin/env bash\n"+body)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// Touch creates empty files, and their directories.
func Touch(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		Write(t, p, "")
	}
}

func Mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

// Read is path's content; a missing file reads as "".
func Read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return string(b)
}

func Exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// Physical resolves symlinks (macOS /var -> /private/var), as cd -P does.
func Physical(t *testing.T, path string) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Lines is the non-empty lines of a log or output.
func Lines(s string) []string {
	var out []string
	for l := range strings.SplitSeq(s, "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// JSONLines decodes each line of a JSONL file.
func JSONLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range Lines(Read(t, path)) {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("%s: %v: %s", path, err, l)
		}
		out = append(out, m)
	}
	return out
}

// Assertions: each reports the run's full output on failure.

func (r Result) Want(t *testing.T, status int) {
	t.Helper()
	if r.Status != status {
		t.Fatalf("status %d, want %d; output:\n%s", r.Status, status, r.Output)
	}
}

func (r Result) Has(t *testing.T, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if !strings.Contains(r.Output, s) {
			t.Errorf("output lacks %q:\n%s", s, r.Output)
		}
	}
}

func (r Result) Lacks(t *testing.T, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if strings.Contains(r.Output, s) {
			t.Errorf("output has %q:\n%s", s, r.Output)
		}
	}
}

func (r Result) Empty(t *testing.T) {
	t.Helper()
	if r.Output != "" {
		t.Errorf("want no output, got:\n%s", r.Output)
	}
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type tee struct {
	own, merged interface{ Write([]byte) (int, error) }
}

func (w *tee) Write(p []byte) (int, error) {
	w.merged.Write(p)
	return w.own.Write(p)
}
