package e2e

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/sources"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
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
	t.Run("SessionStart prunes expired state files and keeps fresh ones", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		old := filepath.Join(e.Claude, "cache", "skills-loaded", "s0-bash-patterns")
		fresh := filepath.Join(e.Claude, "cache", "skills-loaded", "s1-bash-patterns")
		Touch(t, old, fresh)
		testutil.Age(t, 48*time.Hour, old)
		e.run("s1", "").Want(t, 0)
		if Exists(old) || !Exists(fresh) {
			t.Errorf("old kept %v, fresh kept %v; want only the fresh one", Exists(old), Exists(fresh))
		}
	})
	t.Run("SessionStart prunes the repo's gate restore points older than a month", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		tree := e.Git(e.root, "write-tree")
		old := "refs/kit/gates/" + strconv.FormatInt(time.Now().Add(-31*24*time.Hour).UnixNano(), 10)
		fresh := "refs/kit/gates/" + strconv.FormatInt(time.Now().Add(-29*24*time.Hour).UnixNano(), 10)
		e.Git(e.root, "update-ref", old, tree)
		e.Git(e.root, "update-ref", fresh, tree)
		e.run("s1", "").Want(t, 0)
		if refs := e.Git(e.root, "for-each-ref", "--format=%(refname)", "refs/kit/gates/"); refs != fresh {
			t.Errorf("refs = %q, want only %s", refs, fresh)
		}
	})
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

	// <tooling> and the package managers in <repo-context>: what each holds
	// is the hooks and detect packages' tests; this is the wiring.

	t.Run("tooling and repo-context fill in once the background gap-fill run answers", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.useRealKitYML()
		Write(t, filepath.Join(e.root, "package.json"), `{"name":"x","devDependencies":{"turbo":"2.0.0"}}`+"\n")
		Write(t, filepath.Join(e.root, "turbo.json"), `{"tasks":{"lint":{}}}`+"\n")
		Write(t, filepath.Join(e.root, ".gitignore"), "node_modules\n")
		Touch(t, filepath.Join(e.root, "pnpm-lock.yaml"))
		Stub(t, filepath.Join(e.root, "node_modules/.bin/turbo"), "")
		e.Git(e.root, "add", "-A")

		// The fake claude fails: the entry stays unclassified.
		r := e.run("s1", "")
		r.Want(t, 0)
		r.Has(t, "\nunclassified: 1 (")
		r.Lacks(t, "package-manager:", "guidance:")

		lint := sources.Entry{Source: "turbo", File: "turbo.json", Name: "lint", Dir: ".", Body: sources.Body{Text: "turbo run lint"}}
		answer := `{"is_error":false,"subtype":"success","structured_output":{"entries":[{"id":"` + gapfill.Key(lint) + `","role":"check","kind":"lint","mutates":false}],` +
			`"managers":[{"dir":".","cites":"pnpm-lock.yaml","manager":"pnpm"}],"proposals":[]}}`
		classifier := filepath.Join(e.tmp, "classifier")
		Stub(t, classifier, "cat >/dev/null\necho '"+answer+"'\n")
		e.Overlay("classifier: [" + strconv.Quote(classifier) + "]\n")
		e.run("s2", "").Want(t, 0)
		answered := gapfill.CacheFile(filepath.Join(e.Claude, "cache"), e.root)
		for deadline := time.Now().Add(10 * time.Second); !Exists(answered); time.Sleep(50 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatal("SessionStart started no gap-fill run that answered")
			}
		}

		r = e.run("s3", "")
		r.Want(t, 0)
		r.Has(t, "\npackage-manager: pnpm (pnpm-lock.yaml)\n", "\nrun-checks runs:\n  turbo run lint\n", "\nguidance: ")
		r.Lacks(t, "unclassified:")
	})

	t.Run("tooling: a project stating no check gets the discovery instruction and the tools", func(t *testing.T) {
		t.Parallel()
		e := injectContextSetup(t, tree)
		e.useRealKitYML()
		r := e.run("s1", "")
		r.Want(t, 0)
		block := injectContextTooling(r.Output)
		if !strings.Contains(block, "\ndiscovery: nothing in this project states a check") || !strings.Contains(block, "\navailable: ") || strings.Contains(block, "guidance:") {
			t.Errorf("tooling:\n%s", block)
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
