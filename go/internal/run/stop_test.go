package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/resolve"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// check is a Stop gate of kind running body on files, relative to the root.
func (r *repo) check(label, kind, body string, files ...string) enforce.Gate {
	g := enforce.Gate{Label: label, Kinds: []string{kind}, Dir: r.root, Body: body}
	for _, f := range files {
		g.Files = append(g.Files, filepath.Join(r.root, f))
	}
	return g
}

func (r *repo) stop(gates ...enforce.Gate) Checked {
	return Files(enforce.Plan{Root: r.root, Gates: gates}, r.cache, 5*time.Second)
}

func TestAStopPassIsReportedOnly(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "repo")
	out := r.stop(r.check("test", "test", "true"))
	if out.Failed || !strings.Contains(out.Report, "PASS test\n") || !Summary.MatchString(out.Report) {
		t.Errorf("%+v", out)
	}
}

// The notice for the user is the report verbatim: the summary first, then
// each check's line under it.
func TestTheNoticeLeadsWithTheSummary(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "repo")
	skipped := r.check("lint (lint-staged: *.ts)", "lint", "")
	skipped.Skip = "eslint not installed"
	out := r.stop(r.check("test (lefthook.yml: test)", "test", "true"), skipped)
	want := "stop checks: 1 passed, 0 failed, 1 skipped\n" +
		"  PASS test (lefthook.yml: test)\n" +
		"  SKIP lint (lint-staged: *.ts) (eslint not installed)"
	if got := out.Notice(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAFailingTestBlocksWithItsBinaryThenItsOutputTail(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "repo")
	bin := filepath.Join(r.root, "node_modules", ".bin", "vitest")
	testutil.FakeTool(t, bin, filepath.Join(r.aside, "calls"), "seq 1 40; exit 1")
	g := r.check("test", "test", "vitest related {files}", "a.ts")
	g.Bins = []resolve.Resolution{{Path: bin, Source: resolve.Local}}
	r.write("a.ts", "")
	out := r.stop(g)
	has(t, out.Failures, "FAIL test\n  cmd: vitest related a.ts\n  bin: "+bin+" (local)\n11\n", "\n40\n", "checks: 0 passed, 1 failed, 0 skipped")
	lacks(t, out.Failures, "\n10\n")
	if !out.Failed {
		t.Error("didn't block")
	}
}

// a.sh is committed as three lines; the lint stub reports lines 1 and 3.
func TestALintFailsOnlyOnFindingsOnChangedLines(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, change, out string
		has, lacks        []string
		failed            bool
	}{
		{"a finding on a changed line blocks; one on an unchanged line doesn't", "one\ntwo\nTHREE\n", "a.sh:1:1: old\na.sh:3:1: new",
			[]string{"FAIL lint\n", "a.sh:3:1: new", "(1 more on unchanged lines don't block)"}, []string{"a.sh:1:1: old"}, true},
		{"findings only on unchanged lines pass, and say so", "one\ntwo\nthree\nfour\n", "a.sh:1:1: old",
			[]string{"PASS lint (1 finding on unchanged lines)"}, nil, false},
		{"a failure no finding can be read from blocks with its output", "one\ntwo\nTHREE\n", "lint crashed: config not found",
			[]string{"FAIL lint\n", "lint crashed: config not found"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t, "repo")
			r.write("a.sh", "one\ntwo\nthree\n")
			r.commit()
			r.write("a.sh", tt.change)
			testutil.Put(t, r.aside, "out", tt.out+"\n")
			out := r.stop(r.check("lint", "lint", "cat "+filepath.Join(r.aside, "out")+"; exit 1", "a.sh"))
			has(t, out.Report+out.Failures, tt.has...)
			lacks(t, out.Failures, tt.lacks...)
			if out.Failed != tt.failed {
				t.Errorf("failed = %v", out.Failed)
			}
		})
	}
}

func TestEveryLineOfAFileHEADDoesntHaveCounts(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "repo")
	r.commit()
	r.write("new.sh", "one\n")
	out := r.stop(r.check("lint", "lint", "echo 'new.sh:1:1: finding'; exit 1", "new.sh"))
	has(t, out.Failures, "FAIL lint\n", "new.sh:1:1: finding")
}

func TestASkippedCheckIsNamedInTheBlock(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "repo")
	out := r.stop(r.check("typecheck", "typecheck", "no-such-tsc --noEmit"), r.check("test", "test", "echo broken; exit 1"))
	has(t, out.Failures, "SKIP typecheck (not installed: ", "no-such-tsc: command not found)", "FAIL test\n", "checks: 0 passed, 1 failed, 1 skipped")
}

func TestAHungCheckTimesOutAsASkipWithItsChildren(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "repo")
	pid := filepath.Join(r.aside, "pid")
	g := r.check("test", "test", "sleep 60 & echo $! > "+pid+"; wait")
	out := Files(enforce.Plan{Root: r.root, Gates: []enforce.Gate{g}}, r.cache, time.Second)
	has(t, out.Report, "SKIP test (timed out after 1s)")
	n, err := strconv.Atoi(strings.TrimSpace(testutil.Read(t, pid)))
	if err != nil {
		t.Fatal(err)
	}
	// The kill lands as the group dies; give it a moment.
	for range 50 {
		if syscall.Kill(n, 0) != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("the check's child %d outlived it", n)
}

// Each check waits for the other's marker: they pass only side by side.
func TestChecksRunInParallel(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "repo")
	wait := func(mine, theirs string) string {
		return fmt.Sprintf("touch %s; for i in $(seq 100); do [ -e %s ] && exit 0; sleep 0.05; done; exit 1", filepath.Join(r.aside, mine), filepath.Join(r.aside, theirs))
	}
	out := r.stop(r.check("a", "test", wait("a", "b")), r.check("b", "test", wait("b", "a")))
	has(t, out.Report, "PASS a\n", "PASS b\n")
}

func TestACheckThatChangesFilesBlocksAndIsMarked(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "repo")
	r.write("a.ts", "let x\n")
	r.write("b.ts", "let y\n")
	out := r.stop(r.check("lint", "lint", "echo fixed > a.ts", "a.ts"), r.check("test", "test", "true", "b.ts"))
	has(t, out.Failures, "FAIL checks changed a.ts; lint won't run again until kit gates reset")
	if !out.Failed {
		t.Error("didn't block")
	}
	marks := enforce.Marks(r.cache, r.root)
	if len(marks) != 1 {
		t.Fatalf("marks = %+v, want only the check that checked a.ts", marks)
	}
	for _, m := range marks {
		if m.Label != "lint" || !strings.HasPrefix(m.Ref, refPrefix) {
			t.Errorf("mark = %+v", m)
		}
		if _, err := Restore(r.root, m); err != nil {
			t.Fatal(err)
		}
	}
	if data, _ := os.ReadFile(filepath.Join(r.root, "a.ts")); string(data) != "let x\n" {
		t.Errorf("a.ts = %q after restore", data)
	}
}
