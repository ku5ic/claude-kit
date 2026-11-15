package sources

import (
	"maps"
	"slices"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// Every file an entry comes from is one Reads names, so a new reader that
// skips the list fails here, not by going unclassified after an edit.
func TestReadsNamesEveryFileAnEntryComesFrom(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for file, body := range map[string]string{
		".github/workflows/ci.yml": "jobs:\n  test:\n    steps:\n      - run: go test ./...\n",
		".gitlab-ci.yml":           "lint:\n  script:\n    - make lint\n",
		".pre-commit-config.yaml":  "repos:\n  - repo: local\n    hooks:\n      - id: fmt\n        entry: gofmt -l\n",
		".lintstagedrc.json":       `{"*.ts":"eslint"}`,
		".config/lefthook.yml":     "pre-commit:\n  commands:\n    lint:\n      run: eslint {staged_files}\n",
		".husky/pre-push":          "npm test\n",
		".commitlintrc.json":       "{}",
		"turbo.json":               `{"tasks":{"build":{}}}`,
		"nx.json":                  `{"targetDefaults":{"lint":{}}}`,
		"package.json":             `{"scripts":{"test":"vitest run"}}`,
		"Makefile":                 "lint:\n\tgolangci-lint run\n",
		"web/package.json":         `{"scripts":{"lint":"eslint ."}}`,
		"web/.cargo/config.toml":   "[alias]\nci = \"test\"\n",
	} {
		testutil.Put(t, root, file, body)
	}
	sources := map[string]bool{}
	for _, e := range Entries(&config.Config{}, root, []string{".", "web"}) {
		sources[e.Source] = true
		if !Reads(e.File) {
			t.Errorf("%s reads %s, which Reads doesn't name", e.Source, e.File)
		}
	}
	for _, want := range []string{"github-actions", "gitlab-ci", "pre-commit", "lint-staged", "lefthook", "husky", "commitlint", "turbo", "nx", "package-scripts", "make", "cargo-alias"} {
		if !sources[want] {
			t.Errorf("the fixture has no %s entry: %v", want, slices.Sorted(maps.Keys(sources)))
		}
	}
	for _, rel := range []string{"a.ts", "src/main.go", "docs/turbo.json", ".github/ci.yml", "web/.husky/pre-push"} {
		if Reads(rel) {
			t.Errorf("Reads names %s, which no source reads", rel)
		}
	}
}

// An included gitlab file feeds .gitlab-ci.yml's entries, so an edit to it
// reclassifies them; a file nothing reads has no owner.
func TestOwnerNamesTheFileWhoseEntriesAPathFeeds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testutil.Put(t, root, ".gitlab-ci.yml", "include:\n  - local: /ci/lint.yml\n  - ci/jobs/*.yml\n")
	testutil.Put(t, root, "ci/lint.yml", "lint:\n  script:\n    - make lint\n")
	for rel, want := range map[string]string{
		"ci/lint.yml":      ".gitlab-ci.yml",
		"ci/jobs/test.yml": ".gitlab-ci.yml",
		"package.json":     "package.json",
		"ci/other.yml":     "",
		"a.ts":             "",
	} {
		if got := Owner(root, rel); got != want {
			t.Errorf("Owner(%s) = %q, want %q", rel, got, want)
		}
	}
}
