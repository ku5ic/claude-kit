package hooks

import (
	"path/filepath"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestGuardTokens(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (*sandbox, string) {
		k := newSandbox(t)
		dir := repo(t, filepath.Join(k.home, "proj"))
		testutil.Write(t, filepath.Join(dir, "src", "app.css"), "@import \"tailwindcss\";\n@theme { --color-brand-500: #2563eb; }\n")
		return k, filepath.Join(dir, "src", "Card.tsx")
	}
	write := func(path, content string) map[string]any {
		p := editPayload("Write", path, "s", filepath.Dir(path))
		p["tool_input"].(map[string]any)["content"] = content
		return p
	}
	t.Run("an undefined shade blocks", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		r := k.run("guard-dispatch", write(path, `export const Card = () => <div className="bg-primary-600" />`+"\n"))
		r.Want(t, 2)
		r.Has(t, "--color-primary-600, which the project never defines")
	})
	t.Run("a theme color goes through", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		k.run("guard-dispatch", write(path, `export const Card = () => <div className="bg-brand-500 text-gray-900" />`+"\n")).Want(t, 0)
	})
	t.Run("ui-undefined-token in disabled_rules lets it through", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		k.overlay("disabled_rules: [ui-undefined-token]\n")
		k.run("guard-dispatch", write(path, `export const Card = () => <div className="bg-primary-600" />`+"\n")).Want(t, 0)
	})
}
