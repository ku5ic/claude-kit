package extract

import (
	"os"
	"path/filepath"
	"strings"
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
	mk := write("Makefile", "check: lint | test\n\nlint:\n\t@golangci-lint run \\\n\t  ./...\n\ntest: ; go test\n\nci:\n\t$(MAKE) check\n\t-go vet ./...\n")
	just := write("justfile", "check: lint test\n    @echo done\n\nlint:\n    -eslint {{flags}}\n\nsh:\n    #!/usr/bin/env bash\n    cd web\n    eslint .\n\npy:\n    #!/usr/bin/env python3\n    print(1)\n")
	vars := write("vars.mk", "RUN := uv run\nlint:\n\t$(RUN) ruff check .\n\t${RUN} mypy $(SRC)\n")
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
		// A variable the Makefile sets expands; an unset one stays.
		{"make_targets", vars, "", map[string]string{"lint": "uv run ruff check .\nuv run mypy $(SRC)"}, true},
		{"make_targets", oneShell, "", map[string]string{"lint": "cd web\neslint ."}, false},
		{"just_recipes", just, "", map[string]string{"check": "just lint\njust test\necho done", "lint": "eslint $JUST_VAR", "sh": "#!/usr/bin/env bash\ncd web\neslint .", "py": ""}, true},
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

func TestTaskfileAndPreCommit(t *testing.T) {
	dir := t.TempDir()
	taskfile := filepath.Join(dir, "Taskfile.yml")
	precommit := filepath.Join(dir, ".pre-commit-config.yaml")
	for path, body := range map[string]string{
		taskfile: `version: '3'
tasks:
  lint:
    cmds:
      - golangci-lint run {{.PKGS}}
  test:
    deps: [lint, {task: gen}]
    cmd: go test ./...
  gen:
    internal: true
    cmds: [go generate ./...]
  ci:
    cmds:
      - task: lint
      - cmd: go vet ./...
  short: echo hi
`,
		precommit: `repos:
  - repo: https://github.com/astral-sh/ruff-pre-commit
    hooks:
      - id: ruff
        args: [--fix]
      - id: ruff-format
  - repo: local
    hooks:
      - id: pytest
        entry: uv run pytest
        args: [-q]
      - id: ruff
`,
	} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if got := strings.Join(TaskfileTasks(taskfile), " "); got != "lint test ci short" {
		t.Errorf("tasks: %q, want internal gen left out, file order kept", got)
	}
	for name, want := range map[string]string{
		"lint":  "golangci-lint run $TASK_VAR",
		"test":  "task lint\ntask gen\ngo test ./...",
		"ci":    "task lint\ngo vet ./...",
		"short": "echo hi",
	} {
		if got := Bodies("taskfile_tasks", taskfile, "")[name]; got.Text != want || !got.PerLine {
			t.Errorf("taskfile %s: %+v, want %q per line", name, got, want)
		}
	}

	if got := strings.Join(PreCommitHooks(precommit), " "); got != "ruff ruff-format pytest" {
		t.Errorf("hooks: %q, want each id once in file order", got)
	}
	// The first hook with an id wins: its args say what running it does.
	for id, want := range map[string]string{"ruff": "ruff --fix", "ruff-format": "ruff-format", "pytest": "uv run pytest -q"} {
		if got := Bodies("precommit_hooks", precommit, "")[id].Text; got != want {
			t.Errorf("pre-commit %s: %q, want %q", id, got, want)
		}
	}
}

func TestBodiesReadPastALongLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Makefile")
	long := "# " + strings.Repeat("x", 100*1024)
	if err := os.WriteFile(path, []byte(long+"\nlint:\n\teslint .\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Bodies("make_targets", path, "")["lint"].Text; got != "eslint ." {
		t.Errorf("lint after a 100KB line: %q, want %q", got, "eslint .")
	}
}
