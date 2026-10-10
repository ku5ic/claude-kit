package e2e

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Wiring for `kit detect-stack`, against the real kit.yml; HOME is faked
// so the caches land in the test's sandbox.
func TestDetectStack(t *testing.T) {
	t.Parallel()
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

	// What the report holds is the detect package's tests; this is the wiring.
	t.Run("prints the stacks, and the package managers gap-fill verified", func(t *testing.T) {
		k, root := repo(t, "next")
		Write(t, filepath.Join(root, "package.json"), `{"dependencies":{"react":"19.0.0"}}`+"\n")
		Touch(t, filepath.Join(root, "pnpm-lock.yaml"))
		exact(t, k.Run("", "detect-stack"), "root: "+root+"\njs: yes (react)")

		answer := `{"is_error":false,"subtype":"success","structured_output":{"entries":[],"managers":[{"dir":".","cites":"pnpm-lock.yaml","manager":"pnpm"}],"proposals":[]}}`
		classifier := filepath.Join(t.TempDir(), "classifier")
		Stub(t, classifier, "cat >/dev/null\necho '"+answer+"'\n")
		k.Overlay("classifier: [" + strconv.Quote(classifier) + "]\n")
		k.Run("", "enforce", "classify").Want(t, 0)
		r := k.Run("", "detect-stack")
		r.Want(t, 0)
		exact(t, r, "root: "+root+"\njs: yes (react)\npackage-manager: pnpm (pnpm-lock.yaml)")
	})

	// The user overlay at ~/.claude/claude-kit.local.yml merges over kit.yml.

	const customOverlay = `stacks:
  custom:
    sentinels:
      - name: custom.marker
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
		r.Has(t, "js: yes (react,marked)\n")
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
