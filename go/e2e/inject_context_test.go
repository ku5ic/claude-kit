package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// The project is real: a git repo named testproject, resolved from the
// payload's cwd as Claude Code sends it.
type injectContextEnv struct {
	*Kit
	tree, tmp, root string
}

func injectContextSetup(t *testing.T, tree string) *injectContextEnv {
	t.Helper()
	k := NewPlugin(t)
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "testproject")
	Mkdir(t, filepath.Join(k.Claude, "scratch"))
	Mkdir(t, proj)
	k.Git(proj, "init", "-q")
	// The plugin root is the fake .claude, so the rules the hook injects are
	// these copies: plain files, which never count as a leftover link.
	if err := os.CopyFS(filepath.Join(k.Claude, "rules"), os.DirFS(filepath.Join(kitRoot, "rules"))); err != nil {
		t.Fatal(err)
	}
	// Likewise a readable kit.yml; tests that need content overwrite it.
	Touch(t, filepath.Join(k.Claude, "kit.yml"))
	// Physical, as git reports the root the hook resolves.
	return &injectContextEnv{Kit: k, tree: tree, tmp: tmp, root: Physical(t, proj)}
}

func (e *injectContextEnv) kitYML(body string) { Write(e.t, filepath.Join(e.Claude, "kit.yml"), body) }

func (e *injectContextEnv) run(session string) Result {
	e.t.Helper()
	payload, err := json.Marshal(map[string]any{"session_id": session, "cwd": e.root})
	if err != nil {
		e.t.Fatal(err)
	}
	return e.exec(filepath.Join(e.tree, "bin", kitBinName), string(payload), "hook", "inject-context")
}

// `kit hook inject-context` wiring, through the real launcher and a Tree's
// binary: plain context on SessionStart with only git on PATH, and the
// background gap-fill run it starts, which needs the kit binary as its own
// executable. Every block's content is tested in process, in
// internal/hooks.
func TestInjectContext(t *testing.T) {
	t.Parallel()
	tree := Tree(t)

	t.Run("no tool prerequisites: stock bash with no jq or yq on PATH still gets context", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.kitYML("global_skills:\n  - fix-sizing\n")
		bin := filepath.Join(e.tmp, "git-only")
		Mkdir(t, bin)
		git, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(git, filepath.Join(bin, "git")); err != nil {
			t.Fatal(err)
		}
		bash := "/bin/bash"
		if _, err := os.Stat(bash); err != nil {
			if bash, err = exec.LookPath("bash"); err != nil {
				t.Fatal(err)
			}
		}
		e.Setenv("PATH", bin)
		r := e.exec(bash, `{"session_id":"s1","cwd":"`+e.root+`"}`, filepath.Join(tree, "bin", "kit"), "hook", "inject-context")
		r.Want(t, 0)
		r.Lacks(t, "systemMessage")
		r.Has(t, "<required-skills>")
	})

	// In process the hook's own executable is the test binary, so the
	// background run it starts can only be the kit's here.
	t.Run("tooling and repo-context fill in once the background gap-fill run answers", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.kitYML(Read(t, filepath.Join(kitRoot, "kit.yml")))
		Write(t, filepath.Join(e.root, "package.json"), `{"name":"x","devDependencies":{"turbo":"2.0.0"}}`+"\n")
		Write(t, filepath.Join(e.root, "turbo.json"), `{"tasks":{"lint":{}}}`+"\n")
		Write(t, filepath.Join(e.root, ".gitignore"), "node_modules\n")
		Touch(t, filepath.Join(e.root, "pnpm-lock.yaml"))
		Stub(t, filepath.Join(e.root, "node_modules/.bin/turbo"), "")
		e.Git(e.root, "add", "-A")

		// The fake claude fails: the entry stays unclassified.
		r := e.run("s1")
		r.Want(t, 0)
		r.Has(t, "\nunclassified: 1 (")
		r.Lacks(t, "package-manager:", "guidance:")

		lint := sources.Entry{Source: "turbo", File: "turbo.json", Name: "lint", Dir: ".", Body: sources.Body{Text: "turbo run lint"}}
		answer := `{"is_error":false,"subtype":"success","structured_output":{"entries":[{"id":"` + gapfill.Key(lint) + `","role":"check","kind":"lint","mutates":false}],` +
			`"managers":[{"dir":".","cites":"pnpm-lock.yaml","manager":"pnpm"}],"proposals":[]}}`
		classifier := filepath.Join(e.tmp, "classifier")
		Stub(t, classifier, "cat >/dev/null\necho '"+answer+"'\n")
		e.Overlay("classifier: [" + strconv.Quote(classifier) + "]\n")
		e.run("s2").Want(t, 0)
		answered := gapfill.CacheFile(filepath.Join(e.Claude, "cache"), e.root)
		for deadline := time.Now().Add(10 * time.Second); !Exists(answered); time.Sleep(50 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatal("SessionStart started no gap-fill run that answered")
			}
		}

		r = e.run("s3")
		r.Want(t, 0)
		r.Has(t, "\npackage-manager: pnpm (pnpm-lock.yaml)\n", "\nrun-checks runs:\n  turbo run lint\n", "\nguidance: ")
		r.Lacks(t, "unclassified:")
	})
}
