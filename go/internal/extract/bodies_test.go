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
	just := write("justfile", "check: lint test\n    @echo done\n\nlint:\n    -eslint .\n\nsh:\n    #!/usr/bin/env bash\n    cd web\n    eslint .\n\npy:\n    #!/usr/bin/env python3\n    print(1)\n")
	oneShell := write("one.mk", ".ONESHELL:\nlint:\n\tcd web\n\teslint .\n")
	poe := write("pyproject.toml", "[tool.poe.tasks]\nlint = \"ruff check .\"\ntest = {cmd = \"pytest\"}\nall = [\"lint\", \"test\"]\n")

	for _, c := range []struct {
		name, file, arg string
		want            map[string]string
		perLine         bool
	}{
		{"json_keys", pkg, ".scripts", map[string]string{"lint": "eslint .", "test": "vitest run"}, false},
		{"make_targets", mk, "", map[string]string{
			"check": "make lint\nmake test",
			"lint":  "golangci-lint run ./...",
			"test":  "",
			"ci":    "make check\ngo vet ./...",
		}, true},
		{"make_targets", oneShell, "", map[string]string{"lint": "cd web\neslint ."}, false},
		{"just_recipes", just, "", map[string]string{"check": "just lint\njust test\necho done", "lint": "eslint .", "sh": "#!/usr/bin/env bash\ncd web\neslint .", "py": ""}, true},
		// A list joins with spaces (a cargo alias); a poe sequence reads as one
		// unknown command, so it never counts as a gate.
		{"toml_keys", poe, ".tool.poe.tasks", map[string]string{"lint": "ruff check .", "test": "pytest", "all": "lint test"}, false},
	} {
		got := Bodies(c.name, c.file, c.arg)
		if len(got) != len(c.want) {
			t.Errorf("%s: %v, want %q", c.name, got, c.want)
			continue
		}
		for k, v := range c.want {
			if got[k].Text != v {
				t.Errorf("%s %s: %q, want %q", c.name, k, got[k].Text, v)
			}
			// A shebang recipe is one script.
			if want := c.perLine && k != "sh" && k != "py"; got[k].PerLine != want {
				t.Errorf("%s %s: PerLine %v, want %v", c.name, k, got[k].PerLine, want)
			}
		}
	}
	if b := Bodies("regex_lines", mk, "x"); b != nil {
		t.Errorf("regex_lines has bodies: %v", b)
	}
}
