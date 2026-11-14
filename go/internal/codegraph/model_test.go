package codegraph

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// rawDir is a deps out dir holding the fixture's deps and externals.
func rawDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	testutil.Write(t, filepath.Join(dir, "deps.tsv"), testutil.Read(t, "testdata/deps/deps.golden"))
	testutil.Write(t, filepath.Join(dir, "externals.tsv"), testutil.Read(t, "testdata/deps/externals.golden"))
	return dir
}

func TestFingerprintAndC4Golden(t *testing.T) {
	t.Parallel()
	raw := rawDir(t)
	lines, err := Fingerprint(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(lines, "\n")+"\n", testutil.Read(t, "testdata/model/fingerprint.golden"); got != want {
		t.Errorf("fingerprint: got:\n%s\nwant:\n%s", got, want)
	}
	var c4 bytes.Buffer
	if err := C4(raw, "fixture", &c4); err != nil {
		t.Fatal(err)
	}
	if want := testutil.Read(t, "testdata/model/c4.golden"); c4.String() != want {
		t.Errorf("c4: got:\n%s\nwant:\n%s", c4.String(), want)
	}
}

// Names that differ only in punctuation keep a node each.
func TestC4KeepsCollidingNamesApart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testutil.Write(t, filepath.Join(dir, "deps.tsv"), "1\tfoo-bar\tcore\n1\tfoo_bar\tcore\n1\tfoo/bar\tcore\n")
	testutil.Write(t, filepath.Join(dir, "externals.tsv"), "")
	var c4 bytes.Buffer
	if err := C4(dir, "x", &c4); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"m_foo_bar[\"foo-bar\"]", "m_foo_bar_2[\"foo/bar\"]", "m_foo_bar_3[\"foo_bar\"]",
		"m_foo_bar --> m_core", "m_foo_bar_2 --> m_core", "m_foo_bar_3 --> m_core",
	} {
		if !strings.Contains(c4.String(), want) {
			t.Errorf("no %q in:\n%s", want, c4.String())
		}
	}
}

func TestCheckC4(t *testing.T) {
	t.Parallel()
	// model is a model dir holding the skeleton with edit applied, and the
	// fixture's fingerprint.
	model := func(t *testing.T, edit func(string) string) string {
		dir := t.TempDir()
		testutil.Write(t, filepath.Join(dir, "c4-containers.mmd"), edit(testutil.Read(t, "testdata/model/c4.golden")))
		testutil.Write(t, filepath.Join(dir, "fingerprint.txt"), testutil.Read(t, "testdata/model/fingerprint.golden"))
		return dir
	}
	check := func(t *testing.T, dir string) (bool, string) {
		var out bytes.Buffer
		ok, err := CheckC4(dir, rawDir(t), fixtureRepo(t), &out)
		if err != nil {
			t.Fatal(err)
		}
		return ok, out.String()
	}
	same := func(s string) string { return s }

	t.Run("the skeleton checks clean", func(t *testing.T) {
		t.Parallel()
		if ok, out := check(t, model(t, same)); !ok || out != "" {
			t.Errorf("ok %v:\n%s", ok, out)
		}
	})
	for _, tc := range []struct {
		name, from, to, want string
	}{
		{"a removed path", "%% path: internal", "%% path: internal/gone", "gone: m_internal"},
		{"a path outside the repo, though it exists", "%% path: internal", "%% path: ..", "gone: m_internal (c4-containers.mmd) cites .., which is outside the repo"},
		{"an edge with no imports", "  m_cmd --> m_internal\n", "  m_cmd --> m_internal\n  m_cmd --> m_docs\n  m_docs[\"docs\"]\n  %% path: docs\n", "unbacked: m_cmd --> m_docs"},
		{"a chained edge's second hop with no imports", "  m_cmd --> m_internal\n", "  m_docs[\"docs\"]\n  %% path: docs\n  m_cmd --> m_internal --> m_docs\n", "unbacked: m_internal --> m_docs"},
		{"an edge from an inline-labeled node with no imports", "  m_cmd --> m_internal\n", "  m_cmd --> m_internal\n  m_docs[\"docs\"]\n  %% path: docs\n  m_cmd[\"cmd\"] --> m_docs\n", "unbacked: m_cmd --> m_docs"},
		{"an undeclared external", "%% package: react", "%% package: vue", "undeclared: ext_react"},
		{"an external that names no package", "  %% package: react\n", "\n", "unbacked: ext_react"},
	} {
		t.Run(tc.name+" fails", func(t *testing.T) {
			t.Parallel()
			ok, out := check(t, model(t, func(s string) string { return strings.Replace(s, tc.from, tc.to, 1) }))
			if ok || !strings.Contains(out, tc.want) {
				t.Errorf("ok %v, want %q in:\n%s", ok, tc.want, out)
			}
		})
	}
	t.Run("a judgment node is listed, and passes", func(t *testing.T) {
		t.Parallel()
		dir := model(t, func(s string) string { return s + "  stripe[\"Stripe\"]:::ext\n  %% source: judgment\n" })
		if ok, out := check(t, dir); !ok || !strings.Contains(out, `judgment: stripe "Stripe"`) {
			t.Errorf("ok %v:\n%s", ok, out)
		}
	})
	t.Run("a stale fingerprint fails", func(t *testing.T) {
		t.Parallel()
		dir := model(t, same)
		testutil.Write(t, filepath.Join(dir, "fingerprint.txt"), "cmd\nold\n")
		ok, out := check(t, dir)
		if ok || !strings.Contains(out, "stale: internal is new") || !strings.Contains(out, "stale: old is gone") {
			t.Errorf("ok %v:\n%s", ok, out)
		}
	})
	t.Run("no model fails", func(t *testing.T) {
		t.Parallel()
		if ok, out := check(t, t.TempDir()); ok || !strings.HasPrefix(out, "no model: ") {
			t.Errorf("ok %v:\n%s", ok, out)
		}
	})
}

func TestModelDir(t *testing.T) {
	t.Parallel()
	parent := testutil.Physical(t, t.TempDir())
	root := filepath.Join(parent, "work", "app")
	testutil.Mkdir(t, filepath.Join(root, "web", "src"))
	testutil.Git(t, root, "init", "-q")
	home := t.TempDir()
	for _, tc := range []struct{ target, want string }{
		{root, filepath.Join(home, "Arch", "work", "app")},
		{filepath.Join(root, "web"), filepath.Join(home, "Arch", "work", "app", "scopes", "web")},
	} {
		got, err := ModelDir("~/Arch", home, tc.target)
		if err != nil || got != tc.want {
			t.Errorf("%s: %q, %v; want %q", tc.target, got, err, tc.want)
		}
	}
	// Two clones of one origin in one parent keep two models.
	testutil.Git(t, root, "remote", "add", "origin", "git@github.com:acme/app.git")
	fork := filepath.Join(parent, "work", "app-fork")
	testutil.Mkdir(t, fork)
	testutil.Git(t, fork, "init", "-q")
	testutil.Git(t, fork, "remote", "add", "origin", "git@github.com:other/app.git")
	if got, _ := ModelDir("/abs", home, fork); got != filepath.Join("/abs", "work", "app-fork") {
		t.Errorf("a second clone of app: %q", got)
	}
	// A worktree is its main checkout's model.
	testutil.Write(t, filepath.Join(root, "x"), "x\n")
	testutil.Git(t, root, "add", "x")
	testutil.Git(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "x")
	tree := filepath.Join(parent, "trees", "review")
	testutil.Git(t, root, "worktree", "add", "-q", "--detach", tree)
	if got, _ := ModelDir("/abs", home, tree); got != filepath.Join("/abs", "work", "app") {
		t.Errorf("a worktree: %q", got)
	}
	if _, err := ModelDir("/abs", home, t.TempDir()); err == nil {
		t.Error("outside a repo: no error")
	}
	// A base that isn't absolute would put the model under the cwd.
	for _, tc := range []struct{ base, home string }{{"~/Arch", ""}, {"", home}, {"Arch", home}, {"~/Arch", "rel"}} {
		if got, err := ModelDir(tc.base, tc.home, root); err == nil {
			t.Errorf("base %q, home %q: %q, want an error", tc.base, tc.home, got)
		}
	}
}
