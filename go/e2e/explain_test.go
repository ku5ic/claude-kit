package e2e

import (
	"path/filepath"
	"testing"
)

// `kit explain` shows a guard's or the Stop hook's decision and evidence,
// and logs, blocks, and runs nothing.
func TestExplain(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (*Kit, string) {
		k := New(t)
		k.Setenv("CLAUDE_CONFIG_DIR", k.Claude)
		repo := filepath.Join(t.TempDir(), "repo")
		Mkdir(t, repo)
		repo = Physical(t, repo)
		k.Git(repo, "init", "-q", "-b", "main")
		k.Dir = repo
		return k, repo
	}

	t.Run("bash: shows the parse and the block with its rule, and logs nothing", func(t *testing.T) {
		t.Parallel()
		k, _ := setup(t)
		r := k.Run("", "explain", "bash", "cd /tmp && echo x | git push --force origin main")
		r.Want(t, 0)
		r.Has(t, `"echo" "x"  |  "git" "push" "--force" "origin" "main"`, "guard-bash: block  git-force-push")
		if Exists(filepath.Join(k.Claude, "logs/guards.jsonl")) {
			t.Error("explain wrote guards.jsonl")
		}
	})

	t.Run("bash: an ordinary command passes; a lone kit script is allowed", func(t *testing.T) {
		t.Parallel()
		k, _ := setup(t)
		k.Run("", "explain", "bash", "git status").Has(t, "guard-bash: pass")
		k.Run("", "explain", "bash", "kit scratch-dir").Has(t, "guard-bash: allow")
	})

	t.Run("edit: a credential read blocks, a plain write passes", func(t *testing.T) {
		t.Parallel()
		k, repo := setup(t)
		k.Run("", "explain", "edit", filepath.Join(k.Home, ".ssh/id_rsa"), "Read").Has(t, "guard-edit: block  sensitive-read")
		k.Run("", "explain", "edit", filepath.Join(repo, "docs/notes.md")).Has(t, "guard-edit: pass")
		k.Run("", "explain", "edit", filepath.Join(repo, "notes.md")).Has(t, "guard-edit: ask")
	})

	t.Run("stop: names what claims a file, where it runs, and the command", func(t *testing.T) {
		t.Parallel()
		k, repo := setup(t)
		Touch(t, filepath.Join(repo, ".shellcheckrc"))
		Write(t, filepath.Join(repo, "run.sh"), "echo hi\n")
		Write(t, filepath.Join(repo, "notes.md"), "x\n")
		r := k.Run("", "explain", "stop", "run.sh", "notes.md")
		r.Want(t, 0)
		r.Has(t, "shellcheck  (config .shellcheckrc)", "file     run.sh", "unclaimed  notes.md")
	})

	t.Run("stop: names the config a check needs besides where it runs", func(t *testing.T) {
		t.Parallel()
		k, repo := setup(t)
		Write(t, filepath.Join(repo, "go.mod"), "module example.com/x\n")
		Touch(t, filepath.Join(repo, ".golangci.yml"))
		Write(t, filepath.Join(repo, "main.go"), "package main\n")
		k.Run("", "explain", "stop", "main.go").Has(t, "golangci-lint  (config go.mod, needs .golangci.yml)")
	})

	t.Run("stop: with no files, uses the working tree's changes", func(t *testing.T) {
		t.Parallel()
		k, repo := setup(t)
		Touch(t, filepath.Join(repo, ".shellcheckrc"))
		Write(t, filepath.Join(repo, "run.sh"), "echo hi\n")
		k.Run("", "explain", "stop").Has(t, "file     run.sh")
	})
}
