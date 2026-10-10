package e2e

import "testing"

// `kit completion bash|zsh` prints a completion script built from the
// usage text and the hook table. The bash script's behavior and the usage
// errors are tested in cmd/kit.
func TestCompletion(t *testing.T) {
	t.Parallel()
	t.Run("zsh: describes every command, wrapped descriptions joined", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		r := k.Run("", "completion", "zsh")
		r.Want(t, 0)
		r.Has(t, "#compdef kit", "compdef _kit kit",
			"'explain:why a guard or the Stop hook decides what it does; logs, blocks, and runs nothing'",
			"'run-checks:every declared check, in every subproject; exits with the failure count. --plan lists them, with commands, without running any'",
			"'git-base'\n",
			"blast-radius) _files ;;",
			"tasks) _files -/ ;;")
	})
}
