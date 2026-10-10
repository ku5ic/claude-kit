package e2e

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The project is real: a git repo named testproject, resolved from the
// payload's cwd as Claude Code sends it. Tests that pin the <repo-context>
// content write the stack cache themselves (after kit.yml, so it counts as
// fresh); the rest let the hook detect the stack.
type injectContextEnv struct {
	*Kit
	tree, tmp, root, cache string
}

func injectContextSetup(t *testing.T, tree string) *injectContextEnv {
	t.Helper()
	k := NewPlugin(t)
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "testproject")
	Mkdir(t, filepath.Join(k.Claude, "scratch"))
	Mkdir(t, proj)
	k.Git(proj, "init", "-q")
	// Physical, as git reports the root the hook resolves.
	root := Physical(t, proj)
	// The plugin root is the fake .claude, so the rules the hook injects are
	// these copies: plain files, which never count as a leftover link.
	injectContextCopyRules(t, filepath.Join(k.Claude, "rules"))
	// Likewise a readable kit.yml; tests that need content overwrite it.
	Touch(t, filepath.Join(k.Claude, "kit.yml"))
	e := &injectContextEnv{Kit: k, tree: tree, tmp: tmp, root: root}
	e.cache = e.cacheFor("testproject", root)
	Mkdir(t, filepath.Dir(e.cache))
	return e
}

// cacheFor is the stack cache file for a project, as the hook names it (no
// overlay in the fake home, so the .base tag).
func (e *injectContextEnv) cacheFor(name, root string) string {
	sum := sha256.Sum256([]byte(root))
	return filepath.Join(e.Claude, "cache", "stack", name+"-"+hex.EncodeToString(sum[:])[:8]+".base.txt")
}

func (e *injectContextEnv) kitYML(body string) { Write(e.t, filepath.Join(e.Claude, "kit.yml"), body) }

func (e *injectContextEnv) writeCache(lines ...string) {
	Write(e.t, e.cache, strings.Join(lines, "\n")+"\n")
}

// The real kit.yml, so the production provider tables are exercised.
func (e *injectContextEnv) useRealKitYML() { e.kitYML(Read(e.t, filepath.Join(kitRoot, "kit.yml"))) }

func (e *injectContextEnv) run(session, cwd string) Result {
	e.t.Helper()
	if cwd == "" {
		cwd = e.root
	}
	payload, err := json.Marshal(map[string]any{"session_id": session, "cwd": cwd})
	if err != nil {
		e.t.Fatal(err)
	}
	return e.exec(injectContextBin(e.tree), string(payload), "hook", "inject-context")
}

// injectContextTooling is the <tooling> block alone, from the first line to
// the closing tag.
func injectContextTooling(out string) string {
	var block []string
	in := false
	for l := range strings.SplitSeq(out, "\n") {
		if l == "<tooling>" {
			in = true
		}
		if in {
			block = append(block, l)
		}
		if l == "</tooling>" {
			in = false
		}
	}
	return strings.Join(block, "\n")
}

// injectContextIndented is the block's lines starting with two spaces.
func injectContextIndented(block string) string {
	var out []string
	for l := range strings.SplitSeq(block, "\n") {
		if strings.HasPrefix(l, "  ") {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// injectContextCopyRules copies the kit's rules dir to dst.
func injectContextCopyRules(t *testing.T, dst string) {
	t.Helper()
	if err := os.CopyFS(dst, os.DirFS(filepath.Join(kitRoot, "rules"))); err != nil {
		t.Fatal(err)
	}
}

func TestInjectContext(t *testing.T) {
	t.Parallel()
	tree := Tree(t)
	t.Run("<required-skills> contains every global_skills entry", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.kitYML("global_skills:\n  - fix-sizing\n  - context-gathering\nskill_triggers: {}\nstacks: {}\n")
		e.writeCache("root: "+e.root, "js: yes")
		r := e.run("s1", "")
		r.Want(t, 0)
		r.Has(t, "<required-skills>", "fix-sizing", "context-gathering")
	})

	t.Run("<suggested-skills> has one line per detected stack skill with its trigger phrase", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.kitYML(`global_skills:
  - fix-sizing
skill_triggers:
  react-patterns: "before building or restructuring React components"
stacks:
  js:
    skills: [javascript-patterns]
    extras:
      - name: react
        dep: react
        skills: [react-patterns]
`)
		e.writeCache("root: "+e.root, "js: yes (react)")
		r := e.run("s1", "")
		r.Want(t, 0)
		r.Has(t, "<suggested-skills>",
			"before building or restructuring React components: load react-patterns via the Skill tool",
			"load javascript-patterns via the Skill tool")
	})

	t.Run("with guard-skills on, <suggested-skills> adds the file-map skills a repo file matches", func(t *testing.T) {
		yml := `global_skills: []
skill_triggers: {}
skill_file_map:
  - on: basename
    globs: ["*.go"]
    skills: [engineering-fundamentals]
  - on: basename
    globs: ["*.test.*"]
    skills: [test-patterns]
  - on: basename
    globs: ["*.ts"]
    skills: [typescript-patterns]
  - on: basename
    globs: ["*.py"]
    skills: [python-patterns]
stacks:
  go:
    skills: [go-patterns]
`
		for _, guard := range []string{"1", "0"} {
			e := injectContextSetup(t, tree)
			e.kitYML(yml)
			e.writeCache("root: "+e.root, "go: yes")
			Write(t, filepath.Join(e.root, "main.go"), "package main\n")
			Write(t, filepath.Join(e.root, "web", "app.test.js"), "")
			Write(t, filepath.Join(e.root, ".gitignore"), "ignored.py\n")
			Write(t, filepath.Join(e.root, "ignored.py"), "")
			e.Setenv("CLAUDE_GUARD_SKILLS", guard)
			r := e.run("s1", "")
			r.Want(t, 0)
			r.Has(t, "load go-patterns via the Skill tool")
			r.Lacks(t, "typescript-patterns", "python-patterns")
			if guard == "1" {
				r.Has(t, "load engineering-fundamentals via the Skill tool", "load test-patterns via the Skill tool")
			} else {
				r.Lacks(t, "engineering-fundamentals", "test-patterns")
			}
		}
	})

	fileSkillsYML := `global_skills: []
skill_triggers: {}
skill_file_map:
  - on: basename
    globs: ["*.go"]
    skills: [engineering-fundamentals]
  - on: basename
    globs: ["*.py"]
    skills: [python-patterns]
stacks:
  go:
    skills: [go-patterns]
`
	t.Run("a subagent reuses its session's file scan; another session rescans", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.kitYML(fileSkillsYML)
		e.writeCache("root: "+e.root, "go: yes")
		e.Setenv("CLAUDE_GUARD_SKILLS", "1")
		Write(t, filepath.Join(e.root, "main.go"), "package main\n")
		r := e.run("s1", "")
		r.Want(t, 0)
		r.Has(t, "load engineering-fundamentals via the Skill tool")

		Write(t, filepath.Join(e.root, "late.py"), "")
		agent := func(session string) Result {
			return e.exec(injectContextBin(e.tree), `{"session_id":"`+session+`","cwd":"`+e.root+`"}`, "hook", "inject-subagent-context")
		}
		cached := agent("s1")
		cached.Want(t, 0)
		cached.Has(t, "load engineering-fundamentals via the Skill tool")
		cached.Lacks(t, "python-patterns")
		agent("s2").Has(t, "load python-patterns via the Skill tool")
	})

	t.Run("a kit.yml edit after the scan makes the session rescan", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.kitYML("global_skills: []\nskill_triggers: {}\nstacks:\n  go:\n    skills: [go-patterns]\n")
		e.writeCache("root: "+e.root, "go: yes")
		e.Setenv("CLAUDE_GUARD_SKILLS", "1")
		Write(t, filepath.Join(e.root, "tool.py"), "")
		e.run("s1", "").Lacks(t, "python-patterns")

		e.kitYML(fileSkillsYML)
		e.writeCache("root: "+e.root, "go: yes")
		later := time.Now().Add(5 * time.Second)
		for _, f := range []string{filepath.Join(e.Claude, "kit.yml"), e.cache} {
			if err := os.Chtimes(f, later, later); err != nil {
				t.Fatal(err)
			}
		}
		e.run("s1", "").Has(t, "load python-patterns via the Skill tool")
	})

	t.Run("outside a git work tree, a directory walk finds the files", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.kitYML(fileSkillsYML + "skip_dirs: [node_modules, dist]\n")
		e.writeCache("root: "+e.root, "go: yes")
		e.Setenv("CLAUDE_GUARD_SKILLS", "1")
		if err := os.RemoveAll(filepath.Join(e.root, ".git")); err != nil {
			t.Fatal(err)
		}
		Write(t, filepath.Join(e.root, "go.mod"), "module x\n")
		Write(t, filepath.Join(e.root, "cmd", "main.go"), "package main\n")
		Write(t, filepath.Join(e.root, "node_modules", "dep", "x.py"), "")
		Write(t, filepath.Join(e.root, "dist", "gen.py"), "")
		r := e.run("s1", "")
		r.Want(t, 0)
		r.Has(t, "load engineering-fundamentals via the Skill tool")
		r.Lacks(t, "python-patterns")
	})

	t.Run("a non-project context (home) produces no injection", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.kitYML("global_skills:\n  - fix-sizing\n")
		r := e.run("s1", e.Home)
		r.Want(t, 0)
		r.Empty(t)
	})

	t.Run("a suggested skill logs a suggested-skill marker to skills.jsonl", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.kitYML(`global_skills: []
skill_triggers:
  react-patterns: "before building or restructuring React components"
stacks:
  js:
    skills: []
    extras:
      - name: react
        dep: react
        skills: [react-patterns]
`)
		e.writeCache("root: "+e.root, "js: yes (react)")
		e.run("s1", "").Want(t, 0)
		log := filepath.Join(e.Claude, "logs", "skills.jsonl")
		if !Exists(log) {
			t.Fatalf("%s missing", log)
		}
		n := 0
		for _, m := range JSONLines(t, log) {
			if m["event"] == "suggested-skill" && m["session_id"] == "s1" && m["skill_file"] == "react-patterns" {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%d suggested-skill markers, want 1:\n%s", n, Read(t, log))
		}
	})

	// Regression coverage for a fixed bug: inject-context's dirty-file count
	// runs `git -C "$project_root" status --porcelain | wc -l | tr -d ' '`. Under
	// pipefail, a non-git project_root used to make that pipeline fail, and the
	// fail-open ERR trap from kit_hook_init turned that into a silent early exit --
	// no repo-context, no required/suggested skills, no marker touch. Fixed by
	// falling back to a "dirty-files: unknown" line instead of aborting.
	t.Run("a non-git project root degrades to dirty-files: unknown instead of failing open", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		nonGit := filepath.Join(e.tmp, "non-git-project")
		Mkdir(t, nonGit)
		e.kitYML("global_skills:\n  - fix-sizing\n")
		e.cache = e.cacheFor("non-git-project", nonGit)
		e.writeCache("root: "+nonGit, "js: yes")
		r := e.run("s1", nonGit)
		r.Want(t, 0)
		r.Has(t, "<required-skills>", "fix-sizing", "dirty-files (at session start): unknown")
	})

	// Install prerequisites: the kit rules linked and a readable kit.yml. The
	// kit itself needs no jq, yq, or bash 4.

	t.Run("no tool prerequisites: stock bash with no jq or yq on PATH still gets context", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.kitYML("global_skills:\n  - fix-sizing\n")
		bin := filepath.Join(e.tmp, "git-only")
		Mkdir(t, bin)
		git, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(git, filepath.Join(bin, "git")); err != nil {
			t.Fatal(err)
		}
		bash := "/bin/bash"
		if _, err := os.Stat(bash); err != nil {
			if bash, err = exec.LookPath("bash"); err != nil {
				t.Fatal(err)
			}
		}
		// The real launcher, running the freshly built kit, as hooks.json does.
		launcher := filepath.Join(tree, "bin", "kit")
		e.Setenv("PATH", bin)
		r := e.exec(bash, `{"session_id":"s1","cwd":"`+e.root+`"}`, launcher, "hook", "inject-context")
		r.Want(t, 0)
		r.Lacks(t, "systemMessage")
		r.Has(t, "<required-skills>")
	})

	t.Run("prereqs: a missing kit.yml gets a warning naming the plugin fix", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		os.Remove(filepath.Join(e.Claude, "kit.yml"))
		r := e.run("s1", "")
		r.Want(t, 0)
		r.Has(t, "readable kit.yml", "reinstall the plugin")
	})

	t.Run("a kit.yml that fails to load is reported, since every guard then runs with no config", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.kitYML("protected_branches: main\n")
		e.Overlay("global_skills: [fix-sizing]\n")
		r := e.run("s1", "")
		r.Want(t, 0)
		r.Has(t, "systemMessage", "kit.yml", "every guard runs with no config")
		var out struct {
			SystemMessage string `json:"systemMessage"`
		}
		if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(out.SystemMessage, "cannot unmarshal"); n != 1 {
			t.Errorf("the decode error appears %d times, want once:\n%s", n, out.SystemMessage)
		}
		if strings.Contains(out.SystemMessage, "claude-kit.local.yml") {
			t.Errorf("the valid overlay is blamed:\n%s", out.SystemMessage)
		}
	})

	t.Run("an overlay that breaks the merge is reported to the user and to Claude, with the context intact", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.kitYML("global_skills:\n  - fix-sizing\nprotected_branches: [main]\n")
		e.Overlay("protected_branches: main\n")
		r := e.run("s1", "")
		r.Want(t, 0)
		var out struct {
			SystemMessage      string `json:"systemMessage"`
			HookSpecificOutput struct {
				HookEventName     string `json:"hookEventName"`
				AdditionalContext string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
			t.Fatalf("stdout is not one JSON object: %v\n%s", err, r.Stdout)
		}
		if !strings.Contains(out.SystemMessage, "claude-kit.local.yml") || !strings.Contains(out.SystemMessage, "ignored") {
			t.Errorf("systemMessage = %q", out.SystemMessage)
		}
		ctx := out.HookSpecificOutput.AdditionalContext
		if out.HookSpecificOutput.HookEventName != "SessionStart" || !strings.Contains(ctx, "ignored") || !strings.Contains(ctx, "<required-skills>") {
			t.Errorf("hookSpecificOutput = %+v", out.HookSpecificOutput)
		}
	})

	t.Run("a leftover install-rules.sh link is reported, since the rules would load twice", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		clone := filepath.Join(e.tmp, "clone-rules")
		injectContextCopyRules(t, clone)
		if err := os.Symlink(clone, filepath.Join(e.Claude, "rules", "claude-kit")); err != nil {
			t.Fatal(err)
		}
		r := e.run("s1", "")
		r.Want(t, 0)
		r.Has(t, "systemMessage", filepath.Join(e.Claude, "rules", "claude-kit"), "load twice")
	})

	t.Run("a claude-kit link to an older clone missing a rule file is still reported", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		partial := filepath.Join(e.tmp, "partial-rules")
		injectContextCopyRules(t, partial)
		if err := os.Remove(filepath.Join(partial, "workflow.md")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(partial, filepath.Join(e.Claude, "rules", "claude-kit")); err != nil {
			t.Fatal(err)
		}
		e.run("s1", "").Has(t, "load twice")
	})

	t.Run("a claude-kit dir holding none of the kit's rule files is not the kit's", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		Write(t, filepath.Join(e.Claude, "rules", "claude-kit", "my-notes.md"), "mine\n")
		e.run("s1", "").Lacks(t, "load twice")
	})

	t.Run("an unrelated rules dir missing one of the kit's rule files is not the kit's", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		partial := filepath.Join(e.tmp, "partial-rules")
		injectContextCopyRules(t, partial)
		if err := os.Remove(filepath.Join(partial, "workflow.md")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(partial, filepath.Join(e.Claude, "rules", "mine")); err != nil {
			t.Fatal(err)
		}
		e.run("s1", "").Lacks(t, "load twice")
	})

	// <tooling>: run forms from kit.yml's task_providers and toolchain_checks.

	t.Run("tooling: a Python project's justfile recipes are listed as just <recipe>", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.useRealKitYML()
		Write(t, filepath.Join(e.root, "pyproject.toml"), "[project]\nname = \"x\"\n")
		Write(t, filepath.Join(e.root, "justfile"), "test:\n  pytest\nlint:\n  ruff check\n")
		e.Git(e.root, "add", "-A")
		r := e.run("s1", "")
		r.Want(t, 0)
		if want := "tasks:\n  just test\n  just lint"; !strings.Contains(injectContextTooling(r.Output), want) {
			t.Errorf("tooling lacks %q:\n%s", want, r.Output)
		}
	})

	t.Run("tooling: a Rust project lists only its toolchain checks", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.useRealKitYML()
		Write(t, filepath.Join(e.root, "Cargo.toml"), "[package]\nname = \"x\"\n")
		e.Git(e.root, "add", "-A")
		stubs := filepath.Join(e.tmp, "stubs")
		Stub(t, filepath.Join(stubs, "cargo"), "")
		e.PrependPath(stubs)
		r := e.run("s1", "")
		r.Want(t, 0)
		want := "  cargo check\n  cargo clippy -- -D warnings\n  cargo fmt --check\n  cargo test"
		if got := injectContextIndented(injectContextTooling(r.Output)); got != want {
			t.Errorf("tooling lines:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("tooling: an OpenTofu project gets {bin} filled, and validate only after init", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.useRealKitYML()
		Touch(t, filepath.Join(e.root, ".terraform.lock.hcl"))
		e.Git(e.root, "add", "-A")
		stubs := filepath.Join(e.tmp, "stubs")
		Write(t, filepath.Join(stubs, "tofu"), "#!/bin/sh\n")
		if err := os.Chmod(filepath.Join(stubs, "tofu"), 0o755); err != nil {
			t.Fatal(err)
		}
		e.PrependPath(stubs)

		r := e.run("s1", "")
		if got, want := injectContextIndented(injectContextTooling(r.Output)), "  tofu fmt -check -recursive"; got != want {
			t.Errorf("before init:\n%s\nwant:\n%s", got, want)
		}

		Mkdir(t, filepath.Join(e.root, ".terraform"))
		r = e.run("s2", "")
		if got, want := injectContextIndented(injectContextTooling(r.Output)), "  tofu fmt -check -recursive\n  tofu validate"; got != want {
			t.Errorf("after init:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("tooling: lists exactly the toolchain checks run-checks runs", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.kitYML(`toolchain_checks:
  - {stack: js, name: local, cmd: "{bin} --probe", bin: [fakefmt]}
  - {stack: js, name: ambiguous, cmd: "{bin} --probe", bin: [fakeother]}
stacks:
  js:
    sentinels:
      - {name: package.json, anchor: true}
`)
		Write(t, filepath.Join(e.root, "package.json"), "{}\n")
		Write(t, filepath.Join(e.root, ".gitignore"), "node_modules\n")
		Stub(t, filepath.Join(e.root, "node_modules/.bin/fakefmt"), "")
		stubs := filepath.Join(e.tmp, "stubs")
		Stub(t, filepath.Join(stubs, "fakeother"), "")
		e.PrependPath(stubs)
		e.Git(e.root, "add", "-A")

		block := injectContextTooling(e.run("s1", "").Output)
		if got, want := injectContextIndented(block), "  node_modules/.bin/fakefmt --probe"; got != want {
			t.Errorf("tooling lines:\n%s\nwant:\n%s", got, want)
		}
		if !strings.Contains(block, "\nchecks: `kit run-checks --plan` lists what kit run-checks runs") {
			t.Errorf("tooling lacks the --plan pointer:\n%s", block)
		}
		e.Dir = e.root
		r := e.exec(injectContextBin(e.tree), "", "run-checks")
		r.Has(t, "PASS js: local\n", "SKIP js: ambiguous (fakeother only on PATH (")
	})

	t.Run("tooling: leaves out toolchain checks run-checks disables or excludes", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.kitYML(`checks:
  - {name: test, exclude_dirs: [e2e]}
disabled_checks: [off, "kept [e2e]"]
toolchain_checks:
  - {stack: js, name: kept, cmd: "{bin} --kept", bin: [fakefmt]}
  - {stack: js, name: off, cmd: "{bin} --off", bin: [fakefmt]}
  - {stack: js, name: suite, slot: test, cmd: "{bin} --suite", bin: [fakefmt]}
stacks:
  js:
    sentinels:
      - {name: package.json, anchor: true}
`)
		Write(t, filepath.Join(e.root, ".gitignore"), "node_modules\n")
		for _, dir := range []string{e.root, filepath.Join(e.root, "e2e")} {
			Write(t, filepath.Join(dir, "package.json"), "{}\n")
			Stub(t, filepath.Join(dir, "node_modules/.bin/fakefmt"), "")
		}
		e.Git(e.root, "add", "-A")

		// disabled_checks [off] matches "js: off" at the root and in e2e;
		// "kept [e2e]" matches only e2e's. run-checks draws the same line.
		block := injectContextTooling(e.run("s1", "").Output)
		want := "  node_modules/.bin/fakefmt --kept\n  node_modules/.bin/fakefmt --suite"
		if got := injectContextIndented(block); got != want {
			t.Errorf("tooling lines:\n%s\nwant:\n%s", got, want)
		}
		e.Dir = e.root
		e.exec(injectContextBin(e.tree), "", "run-checks", "--plan").Has(t,
			"SKIP js: off (disabled_checks)", "SKIP js: off [e2e] (disabled_checks)", "RUN js: kept\n",
			"SKIP js: kept [e2e] (disabled_checks)", "SKIP js: suite [e2e] (e2e looks like a test suite")
	})

	t.Run("tooling: lists the gates run-checks takes from CI config", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.useRealKitYML()
		Write(t, filepath.Join(e.root, "pyproject.toml"), "[project]\nname = \"x\"\n")
		Write(t, filepath.Join(e.root, ".gitignore"), ".venv\n")
		Write(t, filepath.Join(e.root, ".github/workflows/ci.yml"), "jobs:\n  test:\n    steps:\n      - run: pytest\n")
		Stub(t, filepath.Join(e.root, ".venv/bin/pytest"), "")
		e.Git(e.root, "add", "-A")
		block := injectContextTooling(e.run("s1", "").Output)
		if !strings.Contains(block, "ci gates (from CI config; run-checks runs each whose tool the project has):\n  python: test (.github/workflows/ci.yml: pytest)\n") {
			t.Errorf("tooling lacks the CI gate:\n%s", block)
		}
	})

	t.Run("tooling: workspace packages get their own section with the root's package manager", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.useRealKitYML()
		Write(t, filepath.Join(e.root, "package.json"), `{"name":"root","scripts":{"lint":"eslint ."}}`+"\n")
		Write(t, filepath.Join(e.root, "pnpm-workspace.yaml"), "packages:\n  - \"packages/*\"\n")
		Touch(t, filepath.Join(e.root, "pnpm-lock.yaml"))
		Write(t, filepath.Join(e.root, "packages", "a", "package.json"), `{"name":"a","scripts":{"test":"vitest"}}`+"\n")
		e.Git(e.root, "add", "-A")
		r := e.run("s1", "")
		r.Want(t, 0)
		block := injectContextTooling(r.Output)
		for _, want := range []string{"package-manager: pnpm", "tasks:\n  pnpm run lint", "tasks [packages/a]:\n  pnpm run test", "guidance: "} {
			if !strings.Contains(block, want) {
				t.Errorf("tooling lacks %q:\n%s", want, block)
			}
		}
	})

	t.Run("tooling: tasks with no package manager get no package-manager guidance", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.useRealKitYML()
		Write(t, filepath.Join(e.root, "go.mod"), "module x\n\ngo 1.26\n")
		e.Git(e.root, "add", "-A")
		block := injectContextTooling(e.run("s1", "").Output)
		if !strings.Contains(block, "go vet ./...") || strings.Contains(block, "guidance:") {
			t.Errorf("want go tasks and no guidance:\n%s", block)
		}
	})
	t.Run("tooling: a project with no providers or toolchain gets tools only, no tasks or guidance", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.useRealKitYML()
		r := e.run("s1", "")
		r.Want(t, 0)
		block := injectContextTooling(r.Output)
		for _, bad := range []string{"tasks:", "guidance:"} {
			if strings.Contains(block, bad) {
				t.Errorf("tooling has %q:\n%s", bad, block)
			}
		}
		if !strings.Contains(block, "available: ") && !strings.Contains(block, "missing: ") {
			t.Errorf("tooling lists no tools:\n%s", block)
		}
	})

	t.Run("tooling: tools are split into available and missing by PATH", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.kitYML("tools: [jq, definitely-not-a-real-tool-xyz, git]\n")
		r := e.run("s1", "")
		r.Want(t, 0)
		block := injectContextTooling(r.Output)
		for _, want := range []string{"available: jq, git", "missing: definitely-not-a-real-tool-xyz"} {
			if !strings.Contains(block, want) {
				t.Errorf("tooling lacks %q:\n%s", want, block)
			}
		}
	})

	t.Run("tooling: no tools and no tasks means no block", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		r := e.run("s1", "")
		r.Want(t, 0)
		r.Lacks(t, "<tooling>")
	})

	t.Run("repo-context names the scratch dir without creating it", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.writeCache("root: "+e.root, "js: yes")
		r := e.run("s1", "")
		r.Want(t, 0)
		r.Has(t, "scratch: "+e.root+"/.claude/scratch")
		if Exists(filepath.Join(e.root, ".claude", "scratch")) {
			t.Error("scratch dir was created")
		}
	})
}

// injectContextBin is a Tree's binary: the prerequisite check finds the
// kit's rules at <binary>/../rules, which the harness's kitBin lacks.
func injectContextBin(tree string) string {
	return filepath.Join(tree, "bin", kitBinName)
}
