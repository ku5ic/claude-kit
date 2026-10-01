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
	// The prerequisite check wants the kit rules linked; without this every
	// test would get the warning JSON instead of the context.
	Mkdir(t, filepath.Join(k.Claude, "rules"))
	if err := os.Symlink(filepath.Join(kitRoot, "rules"), filepath.Join(k.Claude, "rules", "kit")); err != nil {
		t.Fatal(err)
	}
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
	for _, l := range strings.Split(out, "\n") {
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
	for _, l := range strings.Split(block, "\n") {
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
	tree := Tree(t, "hooks/inject-context.sh")
	t.Run("<required-skills> contains every global_skills entry", func(t *testing.T) {
		e := injectContextSetup(t, tree)
		e.kitYML("global_skills:\n  - fix-sizing\n  - context-gathering\nskill_triggers: {}\nstacks: {}\n")
		e.writeCache("root: "+e.root, "js: yes")
		r := e.run("s1", "")
		r.Want(t, 0)
		r.Has(t, "<required-skills>", "fix-sizing", "context-gathering")
	})

	t.Run("<suggested-skills> has one line per detected stack skill with its trigger phrase", func(t *testing.T) {
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

	t.Run("a non-project context (home) produces no injection", func(t *testing.T) {
		e := injectContextSetup(t, tree)
		e.kitYML("global_skills:\n  - fix-sizing\n")
		r := e.run("s1", e.Home)
		r.Want(t, 0)
		r.Empty(t)
	})

	t.Run("a suggested skill logs a suggested-skill marker to skills.jsonl", func(t *testing.T) {
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

	// Regression coverage for a fixed bug: inject-context.sh's dirty-file count
	// runs `git -C "$project_root" status --porcelain | wc -l | tr -d ' '`. Under
	// pipefail, a non-git project_root used to make that pipeline fail, and the
	// fail-open ERR trap from kit_hook_init turned that into a silent early exit --
	// no repo-context, no required/suggested skills, no marker touch. Fixed by
	// falling back to a "dirty-files: unknown" line instead of aborting.
	t.Run("a non-git project root degrades to dirty-files: unknown instead of failing open", func(t *testing.T) {
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
		// The real shim and launcher, running the freshly built kit.
		shim := filepath.Join(tree, "hooks", "inject-context.sh")
		e.Setenv("PATH", bin)
		r := e.exec(bash, `{"session_id":"s1","cwd":"`+e.root+`"}`, shim)
		r.Want(t, 0)
		r.Lacks(t, "systemMessage")
		r.Has(t, "<required-skills>")
	})

	t.Run("prereqs: kit rules not linked under ~/.claude/rules gets a warning", func(t *testing.T) {
		e := injectContextSetup(t, tree)
		os.Remove(filepath.Join(e.Claude, "rules", "kit"))
		r := e.run("s1", "")
		r.Want(t, 0)
		r.Has(t, "run "+filepath.Join(e.Root, "install-rules.sh"))
	})

	t.Run("prereqs: a missing kit.yml gets a warning naming the plugin fix", func(t *testing.T) {
		e := injectContextSetup(t, tree)
		os.Remove(filepath.Join(e.Claude, "kit.yml"))
		r := e.run("s1", "")
		r.Want(t, 0)
		r.Has(t, "readable kit.yml", "reinstall the plugin")
	})

	t.Run("prereqs: kit rules linked under another name count", func(t *testing.T) {
		e := injectContextSetup(t, tree)
		rules := filepath.Join(e.Claude, "rules")
		if err := os.Rename(filepath.Join(rules, "kit"), filepath.Join(rules, "claude-kit")); err != nil {
			t.Fatal(err)
		}
		e.kitYML("global_skills:\n  - fix-sizing\n")
		e.writeCache("root: "+e.root, "js: yes")
		r := e.run("s1", "")
		r.Want(t, 0)
		r.Lacks(t, "systemMessage")
		r.Has(t, "<required-skills>")
	})

	t.Run("prereqs: another copy of the kit's rules counts (plugin cache vs marketplace clone)", func(t *testing.T) {
		e := injectContextSetup(t, tree)
		os.Remove(filepath.Join(e.Claude, "rules", "kit"))
		clone := filepath.Join(e.tmp, "clone-rules")
		injectContextCopyRules(t, clone)
		if err := os.Symlink(clone, filepath.Join(e.Claude, "rules", "claude-kit")); err != nil {
			t.Fatal(err)
		}
		r := e.run("s1", "")
		r.Want(t, 0)
		r.Lacks(t, "the kit rules linked")
	})

	t.Run("prereqs: a rules dir missing one of the kit's rule files doesn't count", func(t *testing.T) {
		e := injectContextSetup(t, tree)
		os.Remove(filepath.Join(e.Claude, "rules", "kit"))
		partial := filepath.Join(e.tmp, "partial-rules")
		injectContextCopyRules(t, partial)
		if err := os.Remove(filepath.Join(partial, "workflow.md")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(partial, filepath.Join(e.Claude, "rules", "claude-kit")); err != nil {
			t.Fatal(err)
		}
		e.run("s1", "").Has(t, "the kit rules linked")
	})

	// <tooling>: run forms from kit.yml's task_providers and toolchain_checks.

	t.Run("tooling: a Python project's justfile recipes are listed as just <recipe>", func(t *testing.T) {
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
		e := injectContextSetup(t, tree)
		e.useRealKitYML()
		Write(t, filepath.Join(e.root, "Cargo.toml"), "[package]\nname = \"x\"\n")
		e.Git(e.root, "add", "-A")
		r := e.run("s1", "")
		r.Want(t, 0)
		want := "  cargo check\n  cargo clippy -- -D warnings\n  cargo fmt --check\n  cargo test"
		if got := injectContextIndented(injectContextTooling(r.Output)); got != want {
			t.Errorf("tooling lines:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("tooling: an OpenTofu project gets {bin} filled, and validate only after init", func(t *testing.T) {
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

	t.Run("tooling: workspace packages get their own section with the root's package manager", func(t *testing.T) {
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

	t.Run("tooling: a project with no providers or toolchain gets tools only, no tasks or guidance", func(t *testing.T) {
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
		e := injectContextSetup(t, tree)
		r := e.run("s1", "")
		r.Want(t, 0)
		r.Lacks(t, "<tooling>")
	})

	t.Run("repo-context names the scratch dir without creating it", func(t *testing.T) {
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
