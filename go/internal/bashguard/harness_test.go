package bashguard

import (
	"bytes"
	"encoding/json"
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
	t          *testing.T
	home       string // $HOME, and the cwd of a run that names none
	claude     string // $HOME/.claude
	transcript string // transcript_path of each run; none when ""
}

func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	home := testutil.Physical(t, t.TempDir())
	k := &sandbox{t: t, home: home, claude: filepath.Join(home, ".claude")}
	testutil.Mkdir(t, filepath.Join(k.claude, "logs"))
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
	return k.run(hook.NamedCheck{Name: "guard-bash", Check: Check}, session, cwd, cmd)
}

// commit runs guard-commit on cmd in process.
func (k *sandbox) commit(cwd, cmd string) result {
	k.t.Helper()
	return k.run(hook.NamedCheck{Name: "guard-commit", Check: CheckCommit}, "", cwd, cmd)
}

func (k *sandbox) run(check hook.NamedCheck, session, cwd, cmd string) result {
	k.t.Helper()
	if cwd == "" {
		cwd = k.home
	}
	payload := map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": cmd}, "cwd": cwd}
	if session != "" {
		payload["session_id"] = session
	}
	if k.transcript != "" {
		payload["transcript_path"] = k.transcript
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		k.t.Fatal(err)
	}
	var out bytes.Buffer
	h := &hook.Hook{
		Name:    check.Name,
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
	status := hook.Run(h, check)
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
	testutil.Mkdir(t, dir)
	testutil.Git(t, dir, "init", "-q", "-b", branch)
	return dir
}
