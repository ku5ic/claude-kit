package extract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBodies(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	pkg := write("package.json", `{"scripts":{"lint":"eslint .","test":"vitest run"}}`)
	mk := write("Makefile", "check: lint test\n\nlint:\n\t@golangci-lint run \\\n\t  ./...\n\ntest: ; go test\n\nci:\n\t$(MAKE) check\n\t-go vet ./...\n")
	just := write("justfile", "check: lint test\n    @echo done\n\nlint:\n    -eslint .\n")
	poe := write("pyproject.toml", "[tool.poe.tasks]\nlint = \"ruff check .\"\ntest = {cmd = \"pytest\"}\nall = [\"lint\", \"test\"]\n")

	for _, c := range []struct {
		name, file, arg string
		want            map[string]string
	}{
		{"json_keys", pkg, ".scripts", map[string]string{"lint": "eslint .", "test": "vitest run"}},
		{"make_targets", mk, "", map[string]string{
			"check": "make lint\nmake test",
			"lint":  "golangci-lint run ./...",
			"test":  "",
			"ci":    "make check\ngo vet ./...",
		}},
		{"just_recipes", just, "", map[string]string{"check": "just lint\njust test\necho done", "lint": "eslint ."}},
		// A list joins with spaces (a cargo alias); a poe sequence reads as one
		// unknown command, so it never counts as a gate.
		{"toml_keys", poe, ".tool.poe.tasks", map[string]string{"lint": "ruff check .", "test": "pytest", "all": "lint test"}},
	} {
		got := Bodies(c.name, c.file, c.arg)
		if len(got) != len(c.want) {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
			continue
		}
		for k, v := range c.want {
			if got[k] != v {
				t.Errorf("%s %s: %q, want %q", c.name, k, got[k], v)
			}
		}
	}
	if b := Bodies("regex_lines", mk, "x"); b != nil {
		t.Errorf("regex_lines has bodies: %v", b)
	}
}
