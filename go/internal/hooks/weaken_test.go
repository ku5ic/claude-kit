package hooks

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestGuardWeaken(t *testing.T) {
	t.Parallel()
	const test = "package a\n\nfunc TestA(t *testing.T) {\n\tif got != 1 {\n\t\tt.Errorf(\"got %d\", got)\n\t}\n}\n"
	edit := func(path, old, repl string) map[string]any {
		p := editPayload("Edit", path, "s", filepath.Dir(path))
		p["tool_input"].(map[string]any)["old_string"] = old
		p["tool_input"].(map[string]any)["new_string"] = repl
		return p
	}
	setup := func(t *testing.T) (*sandbox, string) {
		k := newSandbox(t)
		dir := repo(t, filepath.Join(k.home, "proj"))
		path := filepath.Join(dir, "pkg", "a_test.go")
		testutil.Write(t, path, test)
		return k, path
	}
	const skip = "\tt.Skip(\"flaky\")\n\tif got"
	t.Run("an added skip asks, naming no marker for Claude to write", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		r := k.run("guard-dispatch", edit(path, "\tif got", skip))
		r.Want(t, 0)
		r.Has(t, `"permissionDecision":"ask"`, "weakens a check in a_test.go", "skip")
		r.Lacks(t, "kit: allow")
	})
	t.Run("an added skip blocks with weakened_checks.block, as the strict preset sets", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		k.overlay("weakened_checks:\n  block: true\n")
		r := k.run("guard-dispatch", edit(path, "\tif got", skip))
		r.Want(t, 2)
		r.Has(t, "weakens a check in a_test.go", "skip", "kit: allow <reason>")
	})
	t.Run("an added skip goes through quietly with weaken disabled", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		k.overlay("disabled_rules: [weaken]\n")
		r := k.run("guard-dispatch", edit(path, "\tif got", skip))
		r.Allowed(t)
		r.Lacks(t, `"permissionDecision":"ask"`)
	})
	t.Run("an added skip with kit: allow goes through and is logged", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		k.run("guard-dispatch", edit(path, "\tif got", "\t// kit: allow needs docker\n\tt.Skip(\"no docker\")\n\tif got")).Allowed(t)
		if log := read(t, k.paths.LogFile(kitlog.Guards)); !strings.Contains(log, "weaken-allow") || !strings.Contains(log, "needs docker") {
			t.Errorf("guards log lacks the allow:\n%s", log)
		}
	})
	t.Run("a deleted assertion asks", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		r := k.run("guard-dispatch", edit(path, "\t\tt.Errorf(\"got %d\", got)\n", ""))
		r.Want(t, 0)
		r.Has(t, `"permissionDecision":"ask"`, "1 fewer assertion line")
	})
	// JUnit and PHPUnit name their assertions assertX and skip with an
	// annotation or a markTest call, none of which the Go, JS, or Python
	// forms match.
	for _, tc := range []struct{ name, file, body, old, repl, want string }{
		{"a deleted JUnit assertion asks", "src/test/java/FooTest.java",
			"class FooTest {\n  @Test\n  void t() {\n    assertEquals(1, got);\n  }\n}\n",
			"    assertEquals(1, got);\n", "", "1 fewer assertion line"},
		{"a deleted PHPUnit assertion asks", "tests/FooTest.php",
			"<?php\nclass FooTest {\n  public function testX() {\n    $this->assertSame(1, $got);\n  }\n}\n",
			"    $this->assertSame(1, $got);\n", "", "1 fewer assertion line"},
		{"a JUnit @Disabled asks", "src/test/java/FooTest.java",
			"class FooTest {\n  @Test\n  void t() {}\n}\n",
			"  @Test\n", "  @Disabled\n  @Test\n", "skip"},
		{"a JUnit 4 @Ignore asks", "src/test/java/FooTest.java",
			"class FooTest {\n  @Test\n  void t() {}\n}\n",
			"  @Test\n", "  @Ignore\n  @Test\n", "skip"},
		{"a JUnit 5 conditional disable asks", "src/test/java/FooTest.java",
			"class FooTest {\n  @Test\n  void t() {}\n}\n",
			"  @Test\n", "  @DisabledOnOs(OS.LINUX)\n  @Test\n", "skip"},
		{"an ExUnit @tag :skip asks", "test/foo_test.exs",
			"defmodule FooTest do\n  test \"x\" do\n  end\nend\n",
			"  test \"x\" do\n", "  @tag :skip\n  test \"x\" do\n", "skip"},
		{"a Dart skip flag asks", "test/foo_test.dart",
			"void main() {\n  test('x', () {});\n}\n",
			"  test('x', () {});\n", "  test('x', () {}, skip: true);\n", "skip"},
		{"a busted pending asks", "spec/foo_spec.lua",
			"describe('x', function()\n  it('y', function() end)\nend)\n",
			"  it('y', function() end)\n", "  pending('y')\n", "skip"},
		{"a PHPUnit markTestSkipped asks", "tests/FooTest.php",
			"<?php\nclass FooTest {\n  public function testX() {\n  }\n}\n",
			"  public function testX() {\n", "  public function testX() {\n    $this->markTestSkipped('flaky');\n", "skip"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			path := filepath.Join(repo(t, filepath.Join(k.home, "proj")), tc.file)
			testutil.Write(t, path, tc.body)
			r := k.run("guard-dispatch", edit(path, tc.old, tc.repl))
			r.Want(t, 0)
			r.Has(t, `"permissionDecision":"ask"`, tc.want)
		})
	}
	t.Run("an ordinary edit goes through quietly", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		k.run("guard-dispatch", edit(path, "got != 1", "got != 2")).Empty(t)
	})
	// snapshot sets up a repo holding a snapshot, starts a turn as a prompt
	// would, and runs code, which may change files mid-turn, before the edit.
	snapshot := func(t *testing.T, code func(dir string)) result {
		k := newSandbox(t)
		dir := repo(t, filepath.Join(k.home, "proj"))
		snap := filepath.Join(dir, "src", "__snapshots__", "a.test.ts.snap")
		testutil.Write(t, snap, "exports[`a`] = `1`;\n")
		testutil.Write(t, filepath.Join(dir, "src", "a.ts"), "export const a = 1\n")
		k.run("turn-tree", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s", "cwd": dir}).Want(t, 0)
		code(dir)
		return k.run("guard-dispatch", edit(snap, "`1`", "`2`"))
	}
	t.Run("a snapshot rewritten in a turn that changed no code asks", func(t *testing.T) {
		t.Parallel()
		r := snapshot(t, func(string) {})
		r.Want(t, 0)
		r.Has(t, `"permissionDecision":"ask"`, "rewrites the snapshot a.test.ts.snap")
	})
	t.Run("a snapshot rewritten after a test-only change asks", func(t *testing.T) {
		t.Parallel()
		r := snapshot(t, func(dir string) { testutil.Write(t, filepath.Join(dir, "src", "a.test.ts"), "test('a')\n") })
		r.Has(t, `"permissionDecision":"ask"`)
	})
	t.Run("a new snapshot goes through", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		dir := repo(t, filepath.Join(k.home, "proj"))
		k.run("turn-tree", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s", "cwd": dir}).Want(t, 0)
		snap := filepath.Join(dir, "src", "__snapshots__", "b.test.ts.snap")
		call := toolCall("Write", map[string]any{"file_path": snap, "content": "exports[`b`] = `1`;\n"})
		call["cwd"] = dir
		k.run("guard-dispatch", call).Empty(t)
	})
	// In a repo the turn has no record of, the turn's own Edit and Write
	// calls say whether it changed code there.
	elsewhere := func(t *testing.T, edited func(other string) []string) result {
		k := newSandbox(t)
		cwd := repo(t, filepath.Join(k.home, "proj"))
		other := repo(t, filepath.Join(k.home, "other"))
		snap := filepath.Join(other, "src", "__snapshots__", "a.test.ts.snap")
		testutil.Write(t, snap, "exports[`a`] = `1`;\n")
		k.run("turn-tree", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s", "cwd": cwd}).Want(t, 0)
		session := filepath.Join(k.home, "session.jsonl")
		turnIn(t, session, edited(other)...)
		call := edit(snap, "`1`", "`2`")
		call["cwd"], call["transcript_path"] = cwd, session
		return k.run("guard-dispatch", call)
	}
	t.Run("a snapshot in an unrecorded repo, after the turn edited its code, goes through", func(t *testing.T) {
		t.Parallel()
		elsewhere(t, func(other string) []string { return []string{filepath.Join(other, "src", "a.ts")} }).Empty(t)
	})
	t.Run("a snapshot in an unrecorded repo, with no code edit there this turn, asks", func(t *testing.T) {
		t.Parallel()
		r := elsewhere(t, func(string) []string { return []string{"/elsewhere/b.ts"} })
		r.Has(t, `"permissionDecision":"ask"`, "rewrites the snapshot a.test.ts.snap")
	})
	t.Run("a snapshot rewritten after a code change goes through", func(t *testing.T) {
		t.Parallel()
		snapshot(t, func(dir string) { testutil.Write(t, filepath.Join(dir, "src", "a.ts"), "export const a = 2\n") }).Empty(t)
	})
	t.Run("weaken in disabled_rules lets a skip through", func(t *testing.T) {
		t.Parallel()
		k, path := setup(t)
		k.overlay("disabled_rules: [weaken]\n")
		k.run("guard-dispatch", edit(path, "\tif got", "\tt.Skip(\"flaky\")\n\tif got")).Want(t, 0)
	})
}
