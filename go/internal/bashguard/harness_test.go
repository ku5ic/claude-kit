package bashguard

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// kitYML is the repo's kit.yml, the base config every case runs against.
var kitYML, _ = filepath.Abs("../../../kit.yml")

func TestMain(m *testing.M) {
	// Cases build their own repos; git must see only those, never the
	// developer's config or a GIT_DIR inherited from a git hook.
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); strings.HasPrefix(name, "GIT_") {
			os.Unsetenv(name)
		}
	}
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	os.Exit(m.Run())
}

// sandbox is one case's fake $HOME, with Claude's config dir under it
// holding the logs and the overlay, as `kit hook` sees them.
type sandbox struct {
	t      *testing.T
	home   string // $HOME, and the cwd of a run that names none
	claude string // $HOME/.claude
}

func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	home := physical(t, t.TempDir())
	k := &sandbox{t: t, home: home, claude: filepath.Join(home, ".claude")}
	mkdir(t, filepath.Join(k.claude, "logs"))
	return k
}

// result is one run: its exit status (0 allow, 2 block) and its stdout and
// stderr merged.
type result struct {
	Status int
	Output string
}

// hook runs guard-bash on cmd in process, as `kit hook guard-bash` would
// with that payload; an empty cwd is the sandbox's $HOME.
func (k *sandbox) hook(session, cwd, cmd string) result {
	k.t.Helper()
	if cwd == "" {
		cwd = k.home
	}
	payload := map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": cmd}, "cwd": cwd}
	if session != "" {
		payload["session_id"] = session
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		k.t.Fatal(err)
	}
	var out bytes.Buffer
	h := &hook.Hook{
		Name:    "guard-bash",
		Payload: hook.ParsePayload(raw),
		Paths: config.Paths{
			Root:    filepath.Dir(kitYML),
			Home:    k.claude,
			Base:    kitYML,
			Overlay: filepath.Join(k.claude, "claude-kit.local.yml"),
		},
		Stdout: &out,
		Stderr: &out,
		Home:   k.home,
	}
	status := hook.Run(h, hook.NamedCheck{Name: "guard-bash", Check: Check})
	return result{status, out.String()}
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

func (r result) Empty(t *testing.T) {
	t.Helper()
	if r.Output != "" {
		t.Errorf("want no output, got:\n%s", r.Output)
	}
}

// gitRepo makes dir a repository on branch, with no commits.
func gitRepo(t *testing.T, dir, branch string) string {
	t.Helper()
	mkdir(t, dir)
	testutil.Git(t, dir, "init", "-q", "-b", branch)
	return dir
}

func mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	testutil.Put(t, filepath.Dir(path), filepath.Base(path), body)
}

func touch(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		write(t, p, "")
	}
}

// physical resolves symlinks (macOS /var -> /private/var), as cd -P does.
func physical(t *testing.T, path string) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// jsonLines decodes each line of a JSONL file; a missing file has none.
func jsonLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for l := range strings.Lines(string(data)) {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("%s: %v: %s", path, err, l)
		}
		out = append(out, m)
	}
	return out
}
