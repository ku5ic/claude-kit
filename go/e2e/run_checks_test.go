package e2e

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// runChecksEnv is one run-checks test's fixture: a git repo with every file
// added before a run (subprojects come from tracked files), and a stub dir
// first on PATH so tests do not depend on real toolchains.
type runChecksEnv struct {
	t              *testing.T
	k              *Kit
	project, stubs string
}

func runChecksSetup(t *testing.T) *runChecksEnv {
	k := New(t)
	tmp := t.TempDir()
	e := &runChecksEnv{t: t, k: k, project: filepath.Join(tmp, "project"), stubs: filepath.Join(tmp, "stubs")}
	Mkdir(t, e.project)
	Mkdir(t, e.stubs)
	k.Git(e.project, "init", "-q", "-b", "main")
	k.PrependPath(e.stubs)
	k.Dir = e.project
	return e
}

func (e *runChecksEnv) write(rel, body string) { Write(e.t, filepath.Join(e.project, rel), body) }

// stub fakes a tool binary that records "$PWD $*" to <stubs>/<name>.calls.
func (e *runChecksEnv) stub(name string, code int) {
	Stub(e.t, filepath.Join(e.stubs, name),
		fmt.Sprintf("echo \"$PWD $*\" >>%q\nexit %d\n", filepath.Join(e.stubs, name+".calls"), code))
}

// calls is a stub's recorded calls, trailing newlines trimmed as $(cat) does.
func (e *runChecksEnv) calls(name string) string {
	return strings.TrimRight(Read(e.t, filepath.Join(e.stubs, name+".calls")), "\n")
}

func (e *runChecksEnv) called(name string) bool {
	return Exists(filepath.Join(e.stubs, name+".calls"))
}

func (e *runChecksEnv) run(args ...string) Result {
	e.t.Helper()
	e.k.Git(e.project, "add", "-A")
	return e.k.Run("", append([]string{"run-checks"}, args...)...)
}

// Wiring for kit run-checks: what it plans and how it judges a run are the
// enforce and run packages' tests.
func TestRunChecks(t *testing.T) {
	t.Parallel()
	t.Run("a bad argument is a usage error and runs nothing", func(t *testing.T) {
		for _, args := range [][]string{{"--plann"}, {"--only"}, {"--only", "services/nope"}, {"--plan", "extra"}, {"--only", ".", "-x"}} {
			e := runChecksSetup(t)
			e.write("package.json", `{"scripts":{"lint":"eslint ."}}`+"\n")
			e.stub("npm", 0)
			r := e.run(args...)
			r.Want(t, 2)
			r.Has(t, "kit run-checks: ")
			if e.called("npm") {
				t.Errorf("%v ran npm: %s", args, e.calls("npm"))
			}
		}
	})

	t.Run("a flag after --only says where it belongs", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"lint":"eslint ."}}`+"\n")
		r := e.run("--only", ".", "--plan")
		r.Want(t, 2)
		r.Has(t, "kit run-checks: --plan must come before --only")
	})

	t.Run("a relative launcher path from a subdirectory still finds the binary", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("go.mod", "module example.com/fixture\n\ngo 1.22\n")
		Mkdir(t, filepath.Join(e.project, "sub"))
		e.k.Git(e.project, "add", "-A")
		bin := filepath.Join(Tree(t), "bin")
		// ../ up to /, then the launcher's absolute path: relative from sub/.
		sub := Physical(t, filepath.Join(e.project, "sub"))
		up := strings.Repeat("../", strings.Count(sub, "/"))
		launcher := filepath.Join(bin, "kit")
		e.k.Dir = sub
		e.k.Shell("", up+strings.TrimPrefix(launcher, "/")+" run-checks").Has(t, "\nchecks: ")
	})
}
