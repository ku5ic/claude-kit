package hooks

import (
	"path/filepath"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestGuardImports(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (*sandbox, string) {
		k := newSandbox(t)
		dir := repo(t, filepath.Join(k.home, "proj"))
		testutil.Write(t, filepath.Join(dir, "package.json"), `{"name":"app","dependencies":{"react":"^19"}}`)
		return k, filepath.Join(dir, "src", "App.tsx")
	}
	write := func(path, content string) map[string]any {
		p := editPayload("Write", path, "s", filepath.Dir(path))
		p["tool_input"].(map[string]any)["content"] = content
		return p
	}
	t.Run("an undeclared package blocks", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		r := k.run("guard-dispatch", write(path, "import React from 'react'\nimport x from 'made-up-lib'\n"))
		r.Want(t, 2)
		r.Has(t, "imports made-up-lib, which no manifest declares", "package manager")
	})
	t.Run("a declared package goes through", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		k.run("guard-dispatch", write(path, "import React from 'react'\n")).Want(t, 0)
	})
	t.Run("pkg-undeclared in disabled_rules lets it through", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		k.overlay("disabled_rules: [pkg-undeclared]\n")
		k.run("guard-dispatch", write(path, "import x from 'made-up-lib'\n")).Want(t, 0)
	})
}
