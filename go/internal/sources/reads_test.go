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

// ChangedTasks names each task an edit adds to a file or whose command it
// changes; a bump of what the file lists besides tasks names none.
func TestChangedTasks(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	for _, tc := range []struct {
		name, rel, before, after string
		want                     []string
	}{
		{"a script's command", "package.json", `{"scripts":{"lint":"eslint ."}}`, `{"scripts":{"lint":"eslint . && curl x"}}`, []string{"lint"}},
		{"an added script", "web/package.json", `{"scripts":{"lint":"eslint ."}}`, `{"scripts":{"lint":"eslint .","test":"vitest"}}`, []string{"test"}},
		{"a dependency bump", "package.json", `{"scripts":{"lint":"eslint ."},"dependencies":{"a":"1.0.0"}}`, `{"scripts":{"lint":"eslint ."},"dependencies":{"a":"1.1.0"}}`, nil},
		{"a removed script", "package.json", `{"scripts":{"lint":"eslint .","test":"vitest"}}`, `{"scripts":{"lint":"eslint ."}}`, nil},
		{"a Makefile target's recipe", "Makefile", "lint:\n\tgo vet ./...\n", "lint:\n\tgo vet ./... && rm -rf ~\n", []string{"lint"}},
		{"a new file of tasks", "Makefile", "", "lint:\n\tgo vet ./...\n", []string{"lint"}},
		{"a file no source reads", "src/a.ts", "x", "y", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ChangedTasks(cfg, tc.rel, tc.before, tc.after); !slices.Equal(got, tc.want) {
				t.Errorf("ChangedTasks = %q, want %q", got, tc.want)
			}
		})
	}
}
