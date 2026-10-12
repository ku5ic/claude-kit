package e2e

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// `kit hook format-dispatch` wiring: an edited file nothing in the project
// claims takes on_edit's first formatter on PATH, a recording stub here,
// after kit enforce classify cached an empty answer. Which formatter claims
// a file is tested in process, in internal/hooks.
func TestFormatDispatch(t *testing.T) {
	t.Parallel()
	t.Run("unclaimed Markdown takes on_edit's first formatter on PATH", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		tmp := Physical(t, t.TempDir())
		stubs, calls, repo := filepath.Join(tmp, "stubs"), filepath.Join(tmp, "calls"), filepath.Join(tmp, "repo")
		for _, name := range []string{"prettier", "mdformat"} {
			Stub(t, filepath.Join(stubs, name), `echo "`+name+` $*" >>"`+calls+"\"\n")
		}
		// Stubs first, then a bash 4+ ahead of macOS /bin/bash, then git.
		path := []string{stubs}
		for _, tool := range []string{"bash", "git"} {
			if p, err := exec.LookPath(tool); err == nil {
				path = append(path, filepath.Dir(p))
			}
		}
		k.Setenv("PATH", strings.Join(append(path, "/usr/bin", "/bin"), ":"))
		Mkdir(t, repo)
		k.Git(repo, "init", "-q", "-b", "main")

		classifier := filepath.Join(tmp, "classifier")
		Stub(t, classifier, `cat >/dev/null
echo '{"is_error":false,"subtype":"success","structured_output":{"entries":[],"managers":[],"proposals":[]}}'
`)
		k.Overlay("classifier: [" + strconv.Quote(classifier) + "]\n")
		k.Dir = repo
		k.Run("", "enforce", "classify")
		k.Dir = k.Home

		file := filepath.Join(repo, "a.md")
		Write(t, file, "content\n")
		k.Hook("format-dispatch", map[string]any{"tool_input": map[string]any{"file_path": file}}).Want(t, 0)
		if got := strings.TrimRight(Read(t, calls), "\n"); got != "prettier --write "+file {
			t.Errorf("calls %q, want %q", got, "prettier --write "+file)
		}
	})
}
