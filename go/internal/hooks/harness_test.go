package hooks

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/sources"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// kitRoot is the repo root: the real kit.yml, rules, and hooks.json.
var kitRoot, _ = filepath.Abs("../../..")

func TestMain(m *testing.M) {
	// InjectContext starts `<own executable> enforce classify` in the
	// background; in a test that is this binary, which must not rerun the suite.
	if len(os.Args) > 1 && os.Args[1] == "enforce" {
		os.Exit(0)
	}
	// Cases build their own repos and set what they read; nothing comes from
	// the developer's git config or a CLAUDE_* or KIT_* setting of their own.
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "GIT_") || strings.HasPrefix(name, "CLAUDE") || strings.HasPrefix(name, "KIT_") {
			os.Unsetenv(name)
		}
	}
	for name, value := range map[string]string{
		"GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.DevNull,
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t",
	} {
		os.Setenv(name, value)
	}
	os.Exit(m.Run())
}

// entries are this package's hooks by `kit hook` name, run as cmd/kit's
// cmdHook runs them.
var entries = map[string]func(*hook.Hook) int{
	"guard-dispatch":          GuardDispatch,
	"guard-edit":              single("guard-edit", GuardEdit),
	"guard-skills":            single("guard-skills", GuardSkills),
	"log-skills":              single("log-skills", LogSkills),
	"sanitize-output":         single("sanitize-output", SanitizeOutput),
	"format-dispatch":         single("format-dispatch", FormatDispatch),
	"plan-mode-context":       single("plan-mode-context", PlanModeContext),
	"reply-length":            single("reply-length", ReplyLength),
	"review-checks":           single("review-checks", ReviewChecks),
	"stop-checks":             single("stop-checks", StopChecks),
	"inject-context":          single("inject-context", InjectContext),
	"inject-subagent-context": single("inject-subagent-context", InjectSubagentContext),
	"inject-rules":            single("inject-rules", InjectRules),
}

func single(name string, check hook.Check) func(*hook.Hook) int {
	return func(h *hook.Hook) int { return hook.Run(h, hook.NamedCheck{Name: name, Check: check}) }
}

// sandbox is one case's fake $HOME, with Claude's config dir under it
// holding the logs, caches, and overlay, as `kit hook` sees them.
type sandbox struct {
	t      *testing.T
	home   string // $HOME
	claude string // $HOME/.claude
	paths  config.Paths
}

// newSandbox reads the real kit.yml and rules.
func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	home := testutil.Physical(t, t.TempDir())
	claude := filepath.Join(home, ".claude")
	testutil.Mkdir(t, filepath.Join(claude, "logs"))
	return &sandbox{t: t, home: home, claude: claude, paths: config.Paths{
		Root:    kitRoot,
		Home:    claude,
		Base:    filepath.Join(kitRoot, "kit.yml"),
		Overlay: filepath.Join(claude, "claude-kit.local.yml"),
	}}
}

// newPlugin is a sandbox whose plugin root is its .claude, so it reads the
// kit.yml and rules a case writes there.
func newPlugin(t *testing.T) *sandbox {
	t.Helper()
	k := newSandbox(t)
	k.paths.Root = k.claude
	k.paths.Base = filepath.Join(k.claude, "kit.yml")
	return k
}

// kitYML writes the plugin root's kit.yml (newPlugin sandboxes only).
func (k *sandbox) kitYML(body string) {
	k.t.Helper()
	if k.paths.Root == kitRoot {
		k.t.Fatal("kitYML on a sandbox reading the real kit.yml")
	}
	testutil.Write(k.t, k.paths.Base, body)
}

// overlay writes the user overlay, claude-kit.local.yml.
func (k *sandbox) overlay(body string) {
	k.t.Helper()
	testutil.Write(k.t, k.paths.Overlay, body)
}

// result is one run: its exit status, and its output apart and merged in
// the order it was written.
type result struct {
	Status                 int
	Stdout, Stderr, Output string
}

// run runs `kit hook <name> args...` in process with payload: a string as
// is, anything else as JSON.
func (k *sandbox) run(name string, payload any, args ...string) result {
	k.t.Helper()
	raw, ok := payload.(string)
	if !ok {
		data, err := json.Marshal(payload)
		if err != nil {
			k.t.Fatal(err)
		}
		raw = string(data)
	}
	entry := entries[name]
	if entry == nil {
		k.t.Fatalf("no hook %q", name)
	}
	var stdout, stderr, merged bytes.Buffer
	h := &hook.Hook{
		Name:    name,
		Args:    args,
		Payload: hook.ParsePayload([]byte(raw)),
		Paths:   k.paths,
		Stdout:  io.MultiWriter(&stdout, &merged),
		Stderr:  io.MultiWriter(&stderr, &merged),
		Home:    k.home,
	}
	status := entry(h)
	return result{status, stdout.String(), stderr.String(), merged.String()}
}

// classify answers every config entry at root with envelope, as `kit
// enforce classify` does with a classifier stub replaying it, so the
// gap-fill cache holds the verdicts before a hook reads them.
func (k *sandbox) classify(root, envelope string) {
	k.t.Helper()
	answer := filepath.Join(k.t.TempDir(), "answer.json")
	testutil.Write(k.t, answer, envelope)
	k.overlay("classifier: [" + strconv.Quote(testutil.Replayer(k.t, answer)[0]) + "]\n")
	cfg, _, err := sources.LoadConfig(k.paths)
	if err != nil {
		k.t.Fatal(err)
	}
	root, _ = project.Root(root)
	stated := sources.Entries(cfg, root, project.Subprojects(root))
	gapfill.Run(cfg, gapfill.Options{Root: root, CacheDir: k.paths.CacheDir(), Entries: stated, Ask: true, Timeout: time.Minute})
}

// envelope is a classifier answer: entries' verdicts, managers, and
// proposals, each a list of objects.
func envelope(t *testing.T, entries, managers, proposals []map[string]any) string {
	t.Helper()
	orEmpty := func(m []map[string]any) []map[string]any {
		if m == nil {
			return []map[string]any{}
		}
		return m
	}
	data, err := json.Marshal(map[string]any{"is_error": false, "subtype": "success", "structured_output": map[string]any{
		"entries": orEmpty(entries), "managers": orEmpty(managers), "proposals": orEmpty(proposals)}})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// verdict is one entry's answer: its key, and the fields v sets.
func verdict(e sources.Entry, v map[string]any) map[string]any {
	a := map[string]any{"id": gapfill.Key(e), "mutates": false}
	for key, val := range v {
		a[key] = val
	}
	return a
}

// editPayload is a tool call on path: tool_input.file_path, and the
// session and cwd when set.
func editPayload(tool, path, session, cwd string) map[string]any {
	p := map[string]any{"tool_name": tool, "tool_input": map[string]any{"file_path": path}}
	if session != "" {
		p["session_id"] = session
	}
	if cwd != "" {
		p["cwd"] = cwd
	}
	return p
}

func (r result) Want(t *testing.T, status int) {
	t.Helper()
	if r.Status != status {
		t.Fatalf("status %d, want %d; output:\n%s", r.Status, status, r.Output)
	}
}

func (r result) Has(t *testing.T, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if !strings.Contains(r.Output, s) {
			t.Errorf("output lacks %q:\n%s", s, r.Output)
		}
	}
}

func (r result) Lacks(t *testing.T, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if strings.Contains(r.Output, s) {
			t.Errorf("output has %q:\n%s", s, r.Output)
		}
	}
}

func (r result) Empty(t *testing.T) {
	t.Helper()
	if r.Output != "" {
		t.Errorf("want no output, got:\n%s", r.Output)
	}
}

// repo makes dir a git repository with one empty commit, and returns its
// physical path.
func repo(t *testing.T, dir string) string {
	t.Helper()
	testutil.Mkdir(t, dir)
	dir = testutil.Physical(t, dir)
	testutil.Git(t, dir, "init", "-q")
	testutil.Git(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	return dir
}

// gitOut runs git in dir and returns its trimmed output.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := git.Line(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return out
}

// stub writes an executable bash script to path; body follows the shebang.
func stub(t *testing.T, path, body string) {
	t.Helper()
	testutil.Write(t, path, "#!/usr/bin/env bash\n"+body)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// read is path's content; a missing file reads as "".
func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
