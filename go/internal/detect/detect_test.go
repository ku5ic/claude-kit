package detect

import (
	"path/filepath"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// repo is a git repo holding files, every one added, and a cache dir whose
// gap-fill cache holds managers, a JSON list, when it isn't "".
func repo(t *testing.T, managers string, files map[string]string) (root, cacheDir string) {
	t.Helper()
	root, cacheDir = fsx.PhysicalPath(t.TempDir()), t.TempDir()
	testutil.Git(t, root, "init", "-q")
	for name, body := range files {
		testutil.Put(t, root, name, body)
	}
	testutil.Git(t, root, "add", "-A")
	if managers != "" {
		file := gapfill.CacheFile(cacheDir, root)
		testutil.Put(t, filepath.Dir(file), filepath.Base(file), `{"managers":`+managers+`}`)
	}
	return root, cacheDir
}

func TestReport(t *testing.T) {
	t.Parallel()
	cfg := testutil.KitConfig(t)
	const pnpm = `{"dir":".","cites":"pnpm-lock.yaml","manager":"pnpm"}`
	next := map[string]string{
		"package.json":   `{"dependencies":{"next":"15.0.0","react":"19.0.0"},"devDependencies":{"typescript":"5.6.0"}}`,
		"tsconfig.json":  "{}",
		"pnpm-lock.yaml": "",
	}
	for _, c := range []struct {
		name, managers string
		files          map[string]string
		want           string
	}{
		{"a pnpm Next.js repo reports js with extras and its verified manager", "[" + pnpm + "]", next,
			"js: yes (typescript,next,react)\npackage-manager: pnpm (pnpm-lock.yaml)\n"},
		{"a cold cache names no manager", "", next, "js: yes (typescript,next,react)\n"},
		{"a uv Django repo reports python with django and uv", `[{"dir":".","cites":"uv.lock","manager":"uv"}]`,
			map[string]string{"pyproject.toml": "[project]\nname = \"x\"\ndependencies = [\"django>=5.0\"]\n", "uv.lock": "", "manage.py": ""},
			"python: yes (django)\npackage-manager: uv (uv.lock)\n"},
		{"python dependencies are the declared ones dependency_skills names, normalized, in its order", "",
			map[string]string{"Pipfile": "[packages]\nFastAPI = \"*\"\nruff-lsp = \"*\"\n", "requirements-dev.txt": "pytest==8.3\n"},
			"python: yes (pytest,fastapi)\n"},
		{"every subproject is detected, and each lockfile directory's manager", "[" + pnpm + `,{"dir":"services/api","cites":"uv.lock","manager":"uv"}]`,
			map[string]string{
				"package.json": `{"name":"root","private":true}`, "pnpm-workspace.yaml": "packages:\n  - \"packages/*\"\n", "pnpm-lock.yaml": "",
				"packages/a/package.json":     `{"name":"a","dependencies":{"react":"19.0.0"}}`,
				"services/api/pyproject.toml": "[project]\nname = \"api\"\n", "services/api/uv.lock": "",
			},
			"js: yes (react) at ., packages/a\npython: yes at services/api\npackage-manager: pnpm (pnpm-lock.yaml)\npackage-manager [services/api]: uv (uv.lock)\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root, cacheDir := repo(t, c.managers, c.files)
			if got, want := Report(cfg, root, cacheDir), "root: "+root+"\n"+c.want; got != want {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
	t.Run("a repo with no sentinel prints nothing", func(t *testing.T) {
		t.Parallel()
		root, cacheDir := repo(t, "["+pnpm+"]", map[string]string{"pnpm-lock.yaml": "", "notes.txt": ""})
		if got := Report(cfg, root, cacheDir); got != "" {
			t.Errorf("got:\n%s", got)
		}
	})
}
