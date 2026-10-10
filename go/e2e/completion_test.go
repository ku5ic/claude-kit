package e2e

import "testing"

// `kit completion bash|zsh` prints a completion script built from the
// usage text and the hook table.
func TestCompletion(t *testing.T) {
	t.Parallel()
	// complete prints COMPREPLY for the words after `kit`, as readline
	// would call _kit with the cursor on the last one.
	const complete = `source <("$KIT" completion bash)
complete_words() {
  COMP_WORDS=(kit "$@"); COMP_CWORD=$#; COMPREPLY=(); _kit
  echo "${COMPREPLY[*]}"
}
`

	t.Run("bash: completes commands, hook names, and per-command flags", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		r := k.Shell("", complete+`
complete_words sc
complete_words hook guard-
complete_words scratch-rotate --
complete_words hook guard-bash x`)
		r.Want(t, 0)
		if want := "scratch-dir scratch-rotate\nguard-bash guard-commit guard-dispatch guard-edit guard-skills\n--dry-run\n\n"; r.Stdout != want {
			t.Errorf("stdout %q, want %q", r.Stdout, want)
		}
	})

	t.Run("bash: no filename fallback, directories only where only a directory fits", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		dir := t.TempDir()
		Mkdir(t, dir+"/sub")
		Touch(t, dir+"/file.txt")
		r := k.Shell("", complete+`cd '`+dir+`'
complete_words hook gx
complete_words version ''
complete_words tasks ''
complete_words blast-radius f`)
		r.Want(t, 0)
		if want := "\n\nsub\nfile.txt\n"; r.Stdout != want {
			t.Errorf("stdout %q, want %q", r.Stdout, want)
		}
	})

	t.Run("zsh: describes every command, wrapped descriptions joined", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		r := k.Run("", "completion", "zsh")
		r.Want(t, 0)
		r.Has(t, "#compdef kit", "compdef _kit kit",
			"'explain:why a guard or the Stop hook decides what it does; logs, blocks, and runs nothing'",
			"'agent-context:a subagent'\\''s startup context'",
			"'run-checks:every declared check, in every subproject; exits with the failure count. --plan lists them, with commands, without running any'",
			"'git-base'\n",
			"blast-radius) _files ;;",
			"tasks) _files -/ ;;")
	})

	t.Run("a missing or unknown shell is a usage error", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		k.Run("", "completion").Want(t, 2)
		r := k.Run("", "completion", "fish")
		r.Want(t, 2)
		r.Has(t, "usage: kit completion bash|zsh")
	})
}
