package sources

import (
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// line is an Entry.String: source, file, stage, dir, name, files, command.
func line(fields ...string) string { return strings.Join(fields, "\t") }

func TestReaders(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		read  func(string) []Entry
		files map[string]string
		want  []string
	}{
		"pre-commit: one entry per stage, legacy names as git hooks, default_stages for the rest": {
			preCommitEntries,
			map[string]string{".pre-commit-config.yaml": "default_stages: [commit]\nrepos:\n  - repo: https://github.com/astral-sh/ruff-pre-commit\n    rev: v0.6.0\n    hooks:\n      - id: ruff\n        args: [--fix]\n      - id: ruff-format\n        stages: [pre-commit, push]\n  - repo: local\n    hooks:\n      - id: gotest\n        entry: go test ./...\n        pass_filenames: false\n"},
			[]string{
				line("pre-commit", ".pre-commit-config.yaml", "pre-commit", ".", "ruff", "-", "ruff --fix {files}"),
				line("pre-commit", ".pre-commit-config.yaml", "pre-commit", ".", "ruff-format", "-", "ruff-format {files}"),
				line("pre-commit", ".pre-commit-config.yaml", "pre-push", ".", "ruff-format", "-", "ruff-format {files}"),
				line("pre-commit", ".pre-commit-config.yaml", "pre-commit", ".", "gotest", "-", "go test ./..."),
			},
		},
		"pre-commit: a hook naming no stage leaves it to its repo's manifest": {
			preCommitEntries,
			map[string]string{".pre-commit-config.yml": "repos:\n  - repo: https://github.com/commitizen-tools/commitizen\n    hooks:\n      - id: commitizen\n"},
			[]string{line("pre-commit", ".pre-commit-config.yml", "-", ".", "commitizen", "-", "commitizen {files}")},
		},
		"lint-staged: package.json's key, each glob's commands with the files appended": {
			lintStagedEntries,
			map[string]string{"package.json": `{"lint-staged":{"*.ts":["eslint --fix","prettier --write"],"*.md":"prettier --write"}}`},
			[]string{
				line("lint-staged", "package.json", "pre-commit", ".", "*.md", "*.md", "prettier --write {files}"),
				line("lint-staged", "package.json", "pre-commit", ".", "*.ts", "*.ts", "eslint --fix {files}"),
				line("lint-staged", "package.json", "pre-commit", ".", "*.ts", "*.ts", "prettier --write {files}"),
			},
		},
		"lint-staged: a package.json without the key falls through to .lintstagedrc as YAML": {
			lintStagedEntries,
			map[string]string{"package.json": `{"name":"x"}`, ".lintstagedrc": "'*.py': ruff check\n"},
			[]string{line("lint-staged", ".lintstagedrc", "pre-commit", ".", "*.py", "*.py", "ruff check {files}")},
		},
		"lint-staged: a JS config is never read": {
			lintStagedEntries,
			map[string]string{"lint-staged.config.js": "export default {'*.js': 'eslint'}\n"},
			nil,
		},
		"lefthook: commands and scripts by name, then jobs in order, groups inheriting": {
			lefthookEntries,
			map[string]string{"lefthook.yml": "min_version: 1.5.0\npre-commit:\n  commands:\n    lint:\n      glob: \"*.{js,ts}\"\n      root: web/\n      env:\n        TZ: UTC\n      run: eslint {staged_files}\n    off:\n      skip: true\n      run: x\n  scripts:\n    check.sh:\n      runner: bash\ncommit-msg:\n  jobs:\n    - run: commitlint --edit {1}\n    - name: grouped\n      glob: \"*.go\"\n      group:\n        jobs:\n          - name: vet\n            run: go vet {push_files}\n"},
			[]string{
				line("lefthook", "lefthook.yml", "commit-msg", ".", "1", "-", "commitlint --edit {1}"),
				line("lefthook", "lefthook.yml", "commit-msg", ".", "vet", "*.go", "go vet {files}"),
				line("lefthook", "lefthook.yml", "pre-commit", "web", "lint", "*.{js,ts}", "TZ=UTC eslint {files}"),
				line("lefthook", "lefthook.yml", "pre-commit", ".", "check.sh", "-", "bash .lefthook/pre-commit/check.sh"),
			},
		},
		"lefthook: a TOML config, a glob list": {
			lefthookEntries,
			map[string]string{".lefthook.toml": "[pre-push.commands.test]\nglob = [\"*.go\", \"go.mod\"]\nrun = \"go test ./...\"\n"},
			[]string{line("lefthook", ".lefthook.toml", "pre-push", ".", "test", "*.go,go.mod", "go test ./...")},
		},
		"husky: each hook file's lines, without husky's own bootstrap or helpers": {
			huskyEntries,
			map[string]string{
				".husky/pre-commit": "#!/usr/bin/env sh\n. \"$(dirname -- \"$0\")/_/husky.sh\"\n\nnpx lint-staged\n",
				".husky/commit-msg": "npx --no -- commitlint --edit $1\n",
				".husky/_/husky.sh": "echo husky\n",
				".husky/common.sh":  "echo helper\n",
			},
			[]string{
				line("husky", ".husky/commit-msg", "commit-msg", ".", "commit-msg", "-", "npx --no -- commitlint --edit $1"),
				line("husky", ".husky/pre-commit", "pre-commit", ".", "pre-commit", "-", "npx lint-staged"),
			},
		},
		"commitlint: any config, JS included, checks the message at commit-msg": {
			commitlintEntries,
			map[string]string{"package.json": `{"name":"x"}`, "commitlint.config.js": "export default {extends: ['@commitlint/config-conventional']}\n"},
			[]string{line("commitlint", "commitlint.config.js", "commit-msg", ".", "commitlint", "-", "commitlint --edit {files}")},
		},
		"commitlint: package.json's key": {
			commitlintEntries,
			map[string]string{"package.json": `{"commitlint":{"extends":["@commitlint/config-conventional"]}}`},
			[]string{line("commitlint", "package.json", "commit-msg", ".", "commitlint", "-", "commitlint --edit {files}")},
		},
		"commitlint: none configured": {commitlintEntries, map[string]string{"package.json": `{}`}, nil},
		"turbo: tasks and a 1.x pipeline, a package's or the root's task as its task": {
			turboEntries,
			map[string]string{"turbo.json": `{"tasks":{"build":{},"web#lint":{},"//#lint":{}},"pipeline":{"test":{}}}`},
			[]string{
				line("turbo", "turbo.json", "-", ".", "build", "-", "turbo run build"),
				line("turbo", "turbo.json", "-", ".", "lint", "-", "turbo run lint"),
				line("turbo", "turbo.json", "-", ".", "test", "-", "turbo run test"),
			},
		},
		"nx: the affected form of each target, executor defaults skipped": {
			nxEntries,
			map[string]string{"nx.json": `{"targetDefaults":{"build":{},"@nx/jest:jest":{},"test":{}}}`},
			[]string{
				line("nx", "nx.json", "-", ".", "build", "-", "nx affected -t build"),
				line("nx", "nx.json", "-", ".", "test", "-", "nx affected -t test"),
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			for file, body := range c.files {
				testutil.Put(t, root, file, body)
			}
			var got []string
			for _, e := range c.read(root) {
				got = append(got, e.String())
			}
			expect(t, got, c.want...)
		})
	}
}

// Entries reads every source in one order: CI, the hook managers, the task
// graphs, then each subproject's task runners, a disabled one and
// pre-commit's tasks left out (pre-commit is its own source).
func TestEntries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testutil.Put(t, root, ".github/workflows/ci.yml", "jobs:\n  test:\n    env:\n      CGO_ENABLED: \"0\"\n    steps:\n      - uses: actions/checkout@v4\n      - run: |\n          go vet ./...\n          go test ./...\n      - name: lint\n        run: golangci-lint run\n")
	testutil.Put(t, root, ".gitlab-ci.yml", "lint:\n  script:\n    - make lint\n")
	testutil.Put(t, root, ".pre-commit-config.yaml", "repos:\n  - repo: local\n    hooks:\n      - id: fmt\n        entry: gofmt -l\n")
	testutil.Put(t, root, "nx.json", `{"targetDefaults":{"lint":{}}}`)
	testutil.Put(t, root, "package.json", `{"scripts":{"test":"vitest run"}}`)
	testutil.Put(t, root, "Makefile", "lint:\n\tgolangci-lint run\n")
	testutil.Put(t, root, "web/package.json", `{"scripts":{"lint":"eslint ."}}`)
	var got []string
	for _, e := range Entries(&config.Config{DisabledTaskProviders: []string{"make"}}, root, []string{".", "web"}) {
		got = append(got, e.String())
	}
	expect(t, got,
		line("github-actions", ".github/workflows/ci.yml", "-", ".", "test/2", "-", `CGO_ENABLED=0 go vet ./...\ngo test ./...\n`),
		line("github-actions", ".github/workflows/ci.yml", "-", ".", "test/lint", "-", "CGO_ENABLED=0 golangci-lint run"),
		line("gitlab-ci", ".gitlab-ci.yml", "-", ".", "lint", "-", "make lint"),
		line("pre-commit", ".pre-commit-config.yaml", "-", ".", "fmt", "-", "gofmt -l {files}"),
		line("nx", "nx.json", "-", ".", "lint", "-", "nx affected -t lint"),
		line("package-scripts", "package.json", "-", ".", "test", "-", "vitest run"),
		line("package-scripts", "web/package.json", "-", "web", "lint", "-", "eslint ."),
	)
}
