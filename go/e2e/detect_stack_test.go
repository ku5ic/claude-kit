package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Characterization tests for `kit detect-stack`: pins today's report on
// fixtures so later refactors change it only on purpose.
//
// Runs against the real kit.yml; HOME is faked so the stack-list cache
// lands in the test's sandbox.
func TestDetectStack(t *testing.T) {
	output := func(r Result) string { return strings.TrimRight(r.Output, "\n") }
	// repo makes a git repo at name, the cwd of every run, and returns its
	// physical path, which is what git rev-parse (and so the report's root:
	// line) uses.
	repo := func(t *testing.T, name string) (*Kit, string) {
		k := New(t)
		root := filepath.Join(Physical(t, t.TempDir()), name)
		Mkdir(t, root)
		k.Git(root, "init", "-q", "-b", "main")
		k.Dir = root
		return k, root
	}
	exact := func(t *testing.T, r Result, want string) {
		t.Helper()
		if got := output(r); got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	}

	t.Run("pnpm Next.js repo reports js with extras and the pnpm tag", func(t *testing.T) {
		k, root := repo(t, "next")
		Write(t, filepath.Join(root, "package.json"), `{"dependencies":{"next":"15.0.0","react":"19.0.0"},"devDependencies":{"typescript":"5.6.0"}}`+"\n")
		Write(t, filepath.Join(root, "tsconfig.json"), "{}\n")
		Touch(t, filepath.Join(root, "pnpm-lock.yaml"))
		r := k.Run("", "detect-stack")
		r.Want(t, 0)
		exact(t, r, "root: "+root+`
js: yes (typescript,next,react) [pnpm]
versions: react 19.0.0 (declared), next 15.0.0 (declared), typescript 5.6.0 (declared)`)
	})
	t.Run("uv Django repo reports python with django and the uv tag", func(t *testing.T) {
		k, root := repo(t, "django")
		Write(t, filepath.Join(root, "pyproject.toml"), "[project]\nname = \"x\"\ndependencies = [\"django>=5.0\"]\n")
		Touch(t, filepath.Join(root, "uv.lock"), filepath.Join(root, "manage.py"))
		r := k.Run("", "detect-stack")
		r.Want(t, 0)
		exact(t, r, "root: "+root+"\npython: yes (django) [uv]")
	})
	t.Run("python extras read declared dependencies, not a name prefix", func(t *testing.T) {
		k, root := repo(t, "pipenv")
		Write(t, filepath.Join(root, "Pipfile"), "[packages]\nFastAPI = \"*\"\nruff-lsp = \"*\"\n")
		Write(t, filepath.Join(root, "requirements-dev.txt"), "pytest==8.3\n")
		r := k.Run("", "detect-stack")
		r.Want(t, 0)
		exact(t, r, "root: "+root+"\npython: yes (fastapi,pytest)")
	})
	t.Run("repo with no sentinel prints nothing", func(t *testing.T) {
		k, _ := repo(t, "none")
		r := k.Run("", "detect-stack")
		r.Want(t, 0)
		r.Empty(t)
	})
	t.Run("every subproject is detected, each stack with its own ecosystem's manager", func(t *testing.T) {
		k, root := repo(t, "mono")
		Write(t, filepath.Join(root, "package.json"), `{"name":"root","private":true}`+"\n")
		Write(t, filepath.Join(root, "pnpm-workspace.yaml"), "packages:\n  - \"packages/*\"\n")
		Touch(t, filepath.Join(root, "pnpm-lock.yaml"))
		Write(t, filepath.Join(root, "packages/a/package.json"), `{"name":"a","dependencies":{"react":"19.0.0"}}`+"\n")
		Write(t, filepath.Join(root, "services/api/pyproject.toml"), "[project]\nname = \"api\"\n")
		Touch(t, filepath.Join(root, "services/api/uv.lock"))
		k.Git(root, "add", "-A")
		r := k.Run("", "detect-stack")
		r.Want(t, 0)
		exact(t, r, "root: "+root+`
js: yes (react) [pnpm] at ., packages/a
python: yes [uv] at services/api
monorepo: yes (pnpm-workspaces)
versions [packages/a]: react 19.0.0 (declared)`)
	})
	t.Run("versions: installed per subproject, each on its own line", func(t *testing.T) {
		k, root := repo(t, "apps")
		Write(t, filepath.Join(root, "package.json"), `{"name":"root","private":true}`+"\n")
		Write(t, filepath.Join(root, "pnpm-workspace.yaml"), "packages:\n  - \"apps/*\"\n")
		for _, app := range [][2]string{{"admin", "18.3.1"}, {"web", "19.1.0"}} {
			dir := filepath.Join(root, "apps", app[0])
			Write(t, filepath.Join(dir, "package.json"), fmt.Sprintf(`{"name":"%s","dependencies":{"react":"^%s"}}`+"\n", app[0], app[1]))
			Write(t, filepath.Join(dir, "node_modules/react/package.json"), fmt.Sprintf(`{"name":"react","version":"%s"}`+"\n", app[1]))
		}
		r := k.Run("", "detect-stack")
		r.Want(t, 0)
		r.Has(t, "\nversions [apps/admin]: react 18.3.1 (installed)\nversions [apps/web]: react 19.1.0 (installed)")
	})
	t.Run("versions: a workspace-root uv.lock, else a requirements.txt pin", func(t *testing.T) {
		k, root := repo(t, "py")
		Write(t, filepath.Join(root, "pyproject.toml"), "[project]\nname = \"x\"\n")
		Write(t, filepath.Join(root, "uv.lock"), "[[package]]\nname = \"django\"\nversion = \"5.1.2\"\n\n[[package]]\nname = \"pydantic\"\nversion = \"2.9.0\"\n")
		Write(t, filepath.Join(root, "services/api/pyproject.toml"), "[project]\nname = \"api\"\n")
		Write(t, filepath.Join(root, "services/api/requirements.txt"), "FastAPI==0.115.0\n")
		k.Git(root, "add", "-A")
		r := k.Run("", "detect-stack")
		r.Want(t, 0)
		r.Has(t, "versions: django 5.1.2 (locked), pydantic 2.9.0 (locked)\nversions [services/api]: django 5.1.2 (locked), fastapi 0.115.0 (pinned), pydantic 2.9.0 (locked)")
	})

	// The user overlay at ~/.claude/claude-kit.local.yml merges over kit.yml.

	const customOverlay = `stacks:
  custom:
    sentinels:
      - name: custom.marker
    skills: []
`
	t.Run("a stack added by the overlay is detected", func(t *testing.T) {
		k, root := repo(t, "custom")
		Touch(t, filepath.Join(root, "custom.marker"))
		k.Overlay(customOverlay)
		r := k.Run("", "detect-stack")
		r.Want(t, 0)
		exact(t, r, "root: "+root+"\ncustom: yes")
	})
	t.Run("the overlay's arrays append to kit.yml's instead of replacing them", func(t *testing.T) {
		k, root := repo(t, "next")
		Write(t, filepath.Join(root, "package.json"), `{"dependencies":{"react":"19.0.0"}}`+"\n")
		Touch(t, filepath.Join(root, "extra.marker"))
		k.Overlay(`stacks:
  js:
    extras:
      - name: marked
        file: extra.marker
`)
		r := k.Run("", "detect-stack")
		r.Want(t, 0)
		r.Has(t, "js: yes (react,marked) [npm]")
	})
	t.Run("editing the overlay invalidates the merged copy", func(t *testing.T) {
		k, root := repo(t, "custom")
		Touch(t, filepath.Join(root, "custom.marker"))
		k.Overlay("stacks: {}\n")
		k.Run("", "detect-stack").Empty(t)

		k.Overlay(customOverlay)
		// A future mtime: the rewrite can land in the same second as the merge.
		future := time.Date(2099, 1, 1, 0, 0, 0, 0, time.Local)
		if err := os.Chtimes(filepath.Join(k.Claude, "claude-kit.local.yml"), future, future); err != nil {
			t.Fatal(err)
		}
		k.Run("", "detect-stack").Has(t, "custom: yes")
	})
}
