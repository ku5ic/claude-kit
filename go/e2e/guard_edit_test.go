package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuardEdit(t *testing.T) {
	// Each test feeds a synthetic Edit payload to the hook and asserts the
	// exit code: 0 = allow, 2 = block.
	// A leading ~/ is the sandbox HOME.
	for _, tc := range []struct {
		name   string
		path   string
		status int
	}{
		// positive cases (must allow)
		{"allow: ts source file", "/tmp/test.ts", 0},
		{"allow: py source file", "/tmp/test.py", 0},
		{"allow: markdown doc", "/tmp/foo.md", 0},
		{"allow: nested project file", "/tmp/some/nested/dir/file.tsx", 0},
		{"allow: package.json (not a lockfile)", "/tmp/package.json", 0},
		// negative cases (must block): lockfiles
		{"block: package-lock.json", "/tmp/package-lock.json", 2},
		{"block: pnpm-lock.yaml", "/tmp/pnpm-lock.yaml", 2},
		{"block: yarn.lock", "/tmp/yarn.lock", 2},
		{"block: Gemfile.lock", "/tmp/Gemfile.lock", 2},
		{"block: Cargo.lock", "/tmp/Cargo.lock", 2},
		{"block: poetry.lock", "/tmp/poetry.lock", 2},
		{"block: uv.lock", "/tmp/uv.lock", 2},
		// .git/ paths
		{"block: edit inside .git/", "/tmp/repo/.git/HEAD", 2},
		{"block: edit nested inside .git/", "/tmp/repo/.git/refs/heads/main", 2},
		// Shell rc files
		{"block: ~/.zshrc", "~/.zshrc", 2},
		{"block: ~/.zprofile", "~/.zprofile", 2},
		{"block: ~/.bashrc", "~/.bashrc", 2},
		// Guarded lockfiles come from kit.yml's package_managers and extra_lockfiles.
		{"block: bun.lock (a package_managers lockfile the old list missed)", "/tmp/project/bun.lock", 2},
		{"block: Pipfile.lock", "/tmp/project/Pipfile.lock", 2},
		{"block: Cargo.lock (extra_lockfiles)", "/tmp/project/Cargo.lock", 2},
		{"allow: requirements.txt is hand-edited", "/tmp/project/requirements.txt", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := New(t)
			path := tc.path
			if rest, ok := strings.CutPrefix(path, "~/"); ok {
				path = filepath.Join(k.Home, rest)
			}
			k.Hook("guard-edit", Payload("Edit", path, "", "")).Want(t, tc.status)
		})
	}

	// The kit overlay: a write gets a prompt through either path to it.
	// overlay is a fake HOME whose overlay link points at a file in a fake
	// dotfiles tree; it returns the sandbox and the link's source.
	overlay := func(t *testing.T) (*Kit, string) {
		k := New(t)
		src := filepath.Join(Physical(t, t.TempDir()), "dotfiles/claude/claude-kit.local.yml")
		Touch(t, src)
		if err := os.Symlink(src, filepath.Join(k.Claude, "claude-kit.local.yml")); err != nil {
			t.Fatal(err)
		}
		return k, src
	}
	t.Run("ask: Write to the overlay through its ~/.claude link", func(t *testing.T) {
		k, _ := overlay(t)
		r := k.Hook("guard-edit", Payload("Edit", filepath.Join(k.Claude, "claude-kit.local.yml"), "", ""))
		r.Want(t, 0)
		r.Has(t, `"permissionDecision":"ask"`)
	})
	t.Run("ask: Write to the overlay's source file in the dotfiles tree", func(t *testing.T) {
		k, src := overlay(t)
		r := k.Hook("guard-edit", Payload("Edit", src, "", ""))
		r.Want(t, 0)
		r.Has(t, `"permissionDecision":"ask"`)
	})
	t.Run("ask: Edit a CI workflow, which rules/workflow.md says needs confirmation", func(t *testing.T) {
		r := New(t).Hook("guard-edit", Payload("Edit", "/tmp/project/.github/workflows/ci.yml", "", ""))
		r.Want(t, 0)
		r.Has(t, `"permissionDecision":"ask"`, "CI workflow")
	})
	t.Run("allow: Read a CI workflow", func(t *testing.T) {
		r := New(t).Hook("guard-edit", Payload("Read", "/tmp/project/.github/workflows/ci.yml", "", ""))
		r.Want(t, 0)
		r.Empty(t)
	})
	t.Run("allow: a same-named file that is not the overlay", func(t *testing.T) {
		k, _ := overlay(t)
		r := k.Hook("guard-edit", Payload("Edit", filepath.Join(t.TempDir(), "elsewhere/claude-kit.local.yml"), "", ""))
		r.Want(t, 0)
		r.Empty(t)
	})
}
