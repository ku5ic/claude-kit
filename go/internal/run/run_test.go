package run

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/resolve"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// repo is one test's git repo on main, its kit cache, and a directory
// outside it for whatever a gate reads or leaves.
type repo struct {
	t                  *testing.T
	root, cache, aside string
}

func newRepo(t *testing.T, name string) *repo {
	t.Helper()
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := &repo{t: t, root: filepath.Join(tmp, name), cache: filepath.Join(tmp, "cache"), aside: filepath.Join(tmp, "aside")}
	testutil.Put(t, r.aside, ".keep", "")
	testutil.Put(t, r.root, ".keep", "")
	testutil.Git(t, r.root, "init", "-q", "-b", "main")
	return r
}

func (r *repo) write(name, body string) { testutil.Put(r.t, r.root, name, body) }

func (r *repo) commit() {
	r.t.Helper()
	testutil.Git(r.t, r.root, "add", "-A")
	testutil.Git(r.t, r.root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "x")
}

// branchOff commits everything on main and checks out a feature branch, so
// main is the git base and later writes are the change.
func (r *repo) branchOff() {
	r.commit()
	testutil.Git(r.t, r.root, "checkout", "-q", "-b", "feat")
}

func (r *repo) gate(label, body string) enforce.Gate {
	return enforce.Gate{Label: label, Dir: r.root, Body: body}
}

// run runs gates and returns what it printed.
func (r *repo) run(gates ...enforce.Gate) (string, Result) {
	r.t.Helper()
	var out strings.Builder
	res := Gates(enforce.Plan{Root: r.root, Gates: gates}, r.cache, &out)
	return out.String(), res
}

func has(t *testing.T, out string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(out, part) {
			t.Errorf("output lacks %q:\n%s", part, out)
		}
	}
}

func lacks(t *testing.T, out string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if strings.Contains(out, part) {
			t.Errorf("output has %q:\n%s", part, out)
		}
	}
}

// The summary is the line review-checks reads as proof the gate ran.
func TestTheSummaryCountsEachVerdict(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "repo")
	skipped := r.gate("lint", "true")
	skipped.Skip = "unclassified"
	out, res := r.run(r.gate("test", "true"), r.gate("typecheck", "exit 1"), skipped)
	has(t, out, "PASS test\n", "FAIL typecheck\n", "SKIP lint (unclassified)\n", "\nchecks: 1 passed, 1 failed, 1 skipped\n")
	if res != (Result{Pass: 1, Fail: 1, Skip: 1}) {
		t.Errorf("result = %+v", res)
	}
	if !Summary.MatchString(out) {
		t.Errorf("review-checks wouldn't see the summary:\n%s", out)
	}
}

func TestAProjectStatingNothingIsOneSkip(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "repo")
	var out strings.Builder
	Gates(enforce.Plan{Root: r.root, Discovery: "nothing states a check"}, r.cache, &out)
	has(t, out.String(), "SKIP everything (nothing states a check)\n", "checks: 0 passed, 0 failed, 1 skipped")
}

// A path with a space, the repo's or the binary's, stays one word.
func TestAFailureNamesItsBinaryBeforeItsOutput(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "my repo")
	bin := filepath.Join(r.root, "my tools", "bin", "lint")
	testutil.FakeTool(t, bin, filepath.Join(r.aside, "calls"), "echo boom; exit 1")
	g := r.gate("lint", "lint src")
	g.Bins = []resolve.Resolution{{Path: bin, Source: resolve.Local}}
	out, _ := r.run(g)
	has(t, out, "FAIL lint\n  cmd: lint src\n  bin: "+bin+" (local)\nboom\n")
}

func TestAGateRunsWithItsEnvAndNeverInstalls(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "repo")
	g := r.gate("test", `echo "verify=$pnpm_config_verify_deps_before_run mode=$MODE"; exit 1`)
	g.Env = []string{"MODE=ci"}
	out, _ := r.run(g)
	has(t, out, "verify=false mode=ci")
}

func TestACommandNotFoundIsASkip(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "repo")
	out, res := r.run(r.gate("typecheck", "no-such-tool --noEmit"))
	has(t, out, "SKIP typecheck (not installed: ", "no-such-tool: command not found)")
	if res.Skip != 1 {
		t.Errorf("result = %+v", res)
	}
}

func TestBaseIsTheGitBase(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "repo")
	g := r.gate("test", "echo base={base}; exit 1")
	out, _ := r.run(g)
	has(t, out, "SKIP test (no git base)")
	r.branchOff()
	out, _ = r.run(g)
	has(t, out, "  cmd: echo base=main; exit 1\n", "\nbase=main\n")
}

// The plan's own check: a formatter run as a gate on an already-modified
// file is caught, marked, and its file can be put back.
func TestAGateThatChangesFilesFailsAndCanBeUndone(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "repo")
	r.write("a.py", "x=1\n")
	r.commit()
	r.write("a.py", "x = 2\n")
	format := r.gate("format-check", "echo formatted > a.py; echo new > made.txt")
	out, res := r.run(format, r.gate("test", "true"))
	has(t, out, "FAIL format-check\n  cmd: echo formatted > a.py; echo new > made.txt\n  changed: a.py, made.txt; it won't run again until kit gates reset", "PASS test\n")
	if res.Fail != 1 || res.Pass != 1 {
		t.Errorf("result = %+v", res)
	}
	marks := enforce.Marks(r.cache, r.root)
	if len(marks) != 1 {
		t.Fatalf("marks = %+v", marks)
	}
	for _, m := range marks {
		if !strings.HasPrefix(m.Ref, refPrefix) {
			t.Errorf("mark %+v has no restore point", m)
		}
		restored, err := Restore(r.root, m)
		if err != nil || strings.Join(restored, ",") != "a.py" {
			t.Errorf("restored %v, %v", restored, err)
		}
	}
	if got := testutil.Read(t, filepath.Join(r.root, "a.py")); got != "x = 2\n" {
		t.Errorf("a.py = %q, want the edit the gate overwrote", got)
	}
	if testutil.Read(t, filepath.Join(r.root, "made.txt")) != "new\n" {
		t.Error("a file the gate created was touched")
	}
	if staged, _ := git.Lines(r.root, "diff", "--cached", "--name-only"); len(staged) > 0 {
		t.Errorf("restore staged %v", staged)
	}
}

func TestRestorePointsOlderThanAMonthArePruned(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "repo")
	tree, err := git.Snapshot(r.root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	old, fresh := keep(r.root, tree, now.Add(-31*24*time.Hour)), keep(r.root, tree, now.Add(-29*24*time.Hour))
	PruneRestorePoints(r.root, now)
	if refs, _ := git.Lines(r.root, "for-each-ref", "--format=%(refname)", refPrefix); len(refs) != 1 || refs[0] != fresh {
		t.Errorf("refs = %v, want only %s (pruned %s)", refs, fresh, old)
	}
}

func TestFindingsNameExistingFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, f := range []string{"src/my module.ts", "src/old.py", "new.go", "src/new.ts", "src/a.js", "src/a.ts", "src/b.ts", "a.css", "zz.py", "zz.rb", "a.sh", "pkg/c.go"} {
		testutil.Put(t, dir, f, "")
	}
	tests := []struct {
		name, out string
		want      []string // file:line, relative to dir
	}{
		{"a path with a space, after other words", "Unused exports (1)\nfresh  src/my module.ts:1:14", []string{"src/my module.ts:1"}},
		{"path:line: text", "src/old.py:1: unused function 'old' (60% confidence)", []string{"src/old.py:1"}},
		{"path:line:col: text", "new.go:12:6: unreachable func: fresh", []string{"new.go:12"}},
		{"a bare path", "Unused files (1)\nsrc/new.ts", []string{"src/new.ts:0"}},
		{"a file's line, then its findings indented", dir + "/src/a.js\n  3:5  error  no-unused-vars\n  7:1  warning  eqeqeq\n\n1 problem", []string{"src/a.js:3", "src/a.js:7"}},
		{"nothing naming a file", "> knip\n12:30:45 started\nError: Cannot read knip.json\nsrc/gone.ts:1:1", nil},
		// Captured from the real tools (eslint 10, stylelint 17, ruff 0.15,
		// rubocop 1.89, shellcheck 0.11, biome, golangci-lint).
		{"eslint, a message line among a file's findings", "\n" + dir + "/src/a.ts\n  0:0  error  Parsing error: nope\nThe file was not found\n  3:7  warning  Unexpected any  no-explicit-any\n\n\x1b[31m✖ 2 problems\x1b[0m\n", []string{"src/a.ts:0", "src/a.ts:3"}},
		{"stylelint", dir + "/a.css:1:18: Unknown property \"colr\" (property-no-unknown) [error]\n\n1 problem (1 error, 0 warnings)\n", []string{"a.css:1"}},
		{"ruff", "zz.py:2:5: F821 Undefined name `a`\nFound 1 error.\n", []string{"zz.py:2"}},
		{"rubocop, after an indented preamble", "Please also note that you can opt-in:\n  AllCops:\n" + dir + "/zz.rb:1:7: C: [Correctable] Layout/SpaceInsideParens: Space inside parentheses detected.\n", []string{"zz.rb:1"}},
		{"shellcheck", "a.sh:2:6: note: Double quote to prevent globbing. [SC2086]\n", []string{"a.sh:2"}},
		{"biome, a file to format without a line", "src/a.ts:1:1 lint/style/useConst  FIXABLE  ━━━\n  × Use const\nsrc/b.ts format ━━━━━━\n", []string{"src/a.ts:1", "src/b.ts:0"}},
		{"golangci-lint, its source line after", "pkg/c.go:3:2: ineffectual assignment to x (ineffassign)\n\tx := 1\n1 issues:\n", []string{"pkg/c.go:3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, f := range Findings(tt.out, dir) {
				got = append(got, strings.TrimPrefix(f.File, dir+"/")+":"+strconv.Itoa(f.Line))
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("findings = %v, want %v", got, tt.want)
			}
		})
	}
}

// A dead-code gate is judged by its findings on lines changed since the git
// base, never its exit code. Each case commits base on main, branches off,
// and writes change; the gate prints out and exits with code. Every repo
// sets diff.mnemonicPrefix: the changed-lines diff mustn't depend on it.
func TestDeadCodeIsJudgedOnChangedLines(t *testing.T) {
	t.Parallel()
	knipBase := map[string]string{"src/old.ts": "export const old = 1\n"}
	knipChange := map[string]string{"src/new.ts": "export const fresh = 2\n"}
	tests := []struct {
		name         string
		base, change map[string]string
		out          string
		code         int
		has, lacks   []string
	}{
		{"a finding in a changed file fails, one in an unchanged file doesn't", knipBase, knipChange,
			"Unused exports (2)\nold  src/old.ts:1:14\nfresh  src/new.ts:1:14", 1, []string{"FAIL deadcode\n", "fresh  src/new.ts:1:14"}, []string{"old  src/old.ts:1:14"}},
		{"findings only in unchanged files pass, whatever the exit code", knipBase, knipChange,
			"Unused exports (1)\nold  src/old.ts:1:14", 1, []string{"PASS deadcode (1 finding on unchanged lines)"}, nil},
		{"a bare path in an unchanged file passes", knipBase, knipChange,
			"> knip\n\nUnused files (1)\nsrc/old.ts", 1, []string{"PASS deadcode (1 finding on unchanged lines)"}, nil},
		{"a bare path in a changed file fails", knipBase, knipChange,
			"Unused files (1)\nsrc/new.ts\nUnused exports (1)\nold  src/old.ts:1:14", 1, []string{"FAIL deadcode\n", "\nsrc/new.ts\n"}, nil},
		{"nothing parsed and a non-zero exit is a tool error", knipBase, knipChange,
			"> knip\nError: Cannot read knip.json", 2, []string{"FAIL deadcode\n", "Error: Cannot read knip.json"}, nil},
		{"touching a file doesn't inherit its old finding; a new line's finding fails", knipBase,
			map[string]string{"src/old.ts": "export const old = 1\n// touched\nexport const added = 3\n"},
			"Unused exports (2)\nold  src/old.ts:1:14\nadded  src/old.ts:3:14", 1, []string{"FAIL deadcode\n", "added  src/old.ts:3:14"}, []string{"old  src/old.ts:1:14"}},
		{"a touched file's old finding alone passes", knipBase,
			map[string]string{"src/old.ts": "export const old = 1\n// touched\n"},
			"Unused exports (1)\nold  src/old.ts:1:14", 1, []string{"PASS deadcode (1 finding on unchanged lines)"}, nil},
		{"a new line's finding fails in a tracked spaced path", map[string]string{"src/my old.ts": "export const old = 1\n"},
			map[string]string{"src/my old.ts": "export const old = 1\nexport const added = 2\n"},
			"Unused exports (1)\nadded  src/my old.ts:2:14", 1, []string{"FAIL deadcode\n", "added  src/my old.ts:2:14"}, nil},
		{"removed and added lines that look like diff headers stay content", map[string]string{"src/old.ts": "export const old = 1\n-- x\n"},
			map[string]string{"src/old.ts": "export const old = 1\n++ b/src/x.ts\nexport const added = 2\n"},
			"Unused exports (2)\nold  src/old.ts:1:14\nadded  src/old.ts:3:14", 1, []string{"FAIL deadcode\n", "added  src/old.ts:3:14"}, []string{"old  src/old.ts:1:14"}},
		{"a finding in a new file whose path has a space fails", nil, map[string]string{"src/my module.ts": "export const fresh = 2\n"},
			"Unused exports (1)\nfresh  src/my module.ts:1:14", 1, []string{"FAIL deadcode\n"}, nil},
		{"vulture's exit 3 with only unchanged findings passes", map[string]string{"src/old.py": "def old(): pass\n"}, map[string]string{"src/new.py": "def fresh(): pass\n"},
			"src/old.py:1: unused function 'old' (60% confidence)", 3, []string{"PASS deadcode (1 finding on unchanged lines)"}, nil},
		{"deadcode's exit 0 with a finding in a changed file fails", nil, map[string]string{"new.go": "package main\n"},
			"new.go:1:1: unreachable func: fresh", 0, []string{"FAIL deadcode\n", "new.go:1:1: unreachable func: fresh"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t, "repo")
			testutil.Git(t, r.root, "config", "diff.mnemonicPrefix", "true")
			for name, body := range tt.base {
				r.write(name, body)
			}
			r.branchOff()
			for name, body := range tt.change {
				r.write(name, body)
			}
			testutil.Put(t, r.aside, "out", tt.out+"\n")
			g := r.gate("deadcode", "cat "+filepath.Join(r.aside, "out")+"; exit "+strconv.Itoa(tt.code))
			g.Kinds = []string{"deadcode"}
			out, _ := r.run(g)
			has(t, out, tt.has...)
			lacks(t, out, tt.lacks...)
		})
	}
}

func TestDeadCodeRunsOnlyWithAChange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		branchOff bool
		want      string
	}{
		{"with no git base, it's skipped", false, "SKIP deadcode (no git base)"},
		{"with nothing changed since the base, it passes", true, "PASS deadcode (nothing changed since main)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t, "repo")
			if tt.branchOff {
				r.branchOff()
			}
			ran := filepath.Join(r.aside, "ran")
			g := r.gate("deadcode", "touch "+ran)
			g.Kinds = []string{"deadcode"}
			out, _ := r.run(g)
			has(t, out, tt.want)
			if _, err := os.Stat(ran); err == nil {
				t.Error("it ran")
			}
		})
	}
}
