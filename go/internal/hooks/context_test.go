package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestToolingBlock(t *testing.T) {
	t.Parallel()
	const pointer = "`kit run-checks --plan` lists them, with where each comes from, without running them\n"
	gates := []enforce.Gate{
		{Label: "lint (ci.yml: go/vet)", Dir: "/r/go", Body: "go vet ./..."},
		{Label: "lint (package.json: lint)", Dir: "/r", Body: "pnpm run lint"},
		{Label: "deadcode (ci.yml: go/deadcode)", Dir: "/r/go", Body: "out=\"$(deadcode ./...)\"\n[ -z \"$out\" ]\n"},
		{Label: "test (ci.yml: shell/bats)", Dir: "/r", Body: "bats test", Skip: "bats only on PATH"},
	}
	for _, c := range []struct {
		name    string
		plan    enforce.Plan
		managed bool
		cli     []string
		want    string
	}{
		{"what run-checks runs, root first, a multi-line body by its label", enforce.Plan{Root: "/r", Gates: gates}, false, nil,
			"run-checks runs:\n  pnpm run lint\nrun-checks runs [go]:\n  go vet ./...\n  deadcode (ci.yml: go/deadcode)\n" + pointer},
		{"unclassified entries", enforce.Plan{Root: "/r", Unclassified: 2}, false, []string{"available: rg"},
			"unclassified: 2 (config entries gap-fill hasn't answered; it runs in the background, and Stop skips them until it answers)\n" + pointer + "available: rg\n"},
		{"nothing stated", enforce.Plan{Root: "/r", Discovery: enforce.Discovery}, false, nil, "discovery: " + enforce.Discovery + "\n"},
		{"guidance only with a verified package manager", enforce.Plan{Root: "/r", Gates: gates[1:2]}, true, nil,
			"run-checks runs:\n  pnpm run lint\n" + pointer + "\nguidance: " + guidance + "\n"},
		{"tools only", enforce.Plan{Root: "/r"}, false, []string{"missing: sg"}, "missing: sg\n"},
		{"nothing at all", enforce.Plan{Root: "/r"}, false, nil, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			want := c.want
			if want != "" {
				want = "\n<tooling>\n" + want + "</tooling>\n"
			}
			if got := toolingBlock(c.plan, c.managed, c.cli); got != want {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
	t.Run("subprojects past 20 are capped with a note", func(t *testing.T) {
		t.Parallel()
		var many []enforce.Gate
		for i := range 22 {
			many = append(many, enforce.Gate{Dir: fmt.Sprintf("/r/p%02d", i), Body: "make test"})
		}
		got := toolingBlock(enforce.Plan{Root: "/r", Gates: many}, false, nil)
		if strings.Count(got, "run-checks runs [") != 20 || !strings.Contains(got, "\n(subprojects capped at 20; kit run-checks covers all)\n") {
			t.Errorf("got:\n%s", got)
		}
	})
}

// contextEnv is one inject-context case: a plugin sandbox reading an empty
// kit.yml and copies of the kit's rules (plain files, never a leftover
// link), and a git repo named testproject. Cases that pin <repo-context>
// write the stack cache themselves, after kit.yml, so it counts as fresh.
type contextEnv struct {
	*sandbox
	tmp, root string
}

func contextSetup(t *testing.T) *contextEnv {
	t.Helper()
	e := &contextEnv{sandbox: newPlugin(t), tmp: t.TempDir()}
	testutil.Mkdir(t, filepath.Join(e.claude, "scratch"))
	proj := filepath.Join(e.tmp, "testproject")
	testutil.Mkdir(t, proj)
	testutil.Git(t, proj, "init", "-q")
	// Physical, as git reports the root the hook resolves.
	e.root = testutil.Physical(t, proj)
	copyRules(t, filepath.Join(e.claude, "rules"))
	e.kitYML("")
	return e
}

// copyRules copies the kit's rules dir to dst.
func copyRules(t *testing.T, dst string) {
	t.Helper()
	if err := os.CopyFS(dst, os.DirFS(filepath.Join(kitRoot, "rules"))); err != nil {
		t.Fatal(err)
	}
}

// stackCache is the stack report file for the project name at root (no
// overlay, so the base tag).
func (e *contextEnv) stackCache(name, root string) string {
	return filepath.Join(e.claude, "cache", cache.Stack, name+"-"+cache.RootKey(root)+".base.txt")
}

// writeCache writes testproject's stack report.
func (e *contextEnv) writeCache(lines ...string) {
	testutil.Write(e.t, e.stackCache("testproject", e.root), strings.Join(lines, "\n")+"\n")
}

func (e *contextEnv) useRealKitYML() { e.kitYML(testutil.Read(e.t, filepath.Join(kitRoot, "kit.yml"))) }

// inject runs inject-context; an empty cwd is the project's root.
func (e *contextEnv) inject(session, cwd string) result {
	e.t.Helper()
	if cwd == "" {
		cwd = e.root
	}
	return e.run("inject-context", map[string]any{"session_id": session, "cwd": cwd})
}

// sessionStart is inject-context's JSON object, printed when it has
// install problems to report.
type sessionStart struct {
	SystemMessage      string `json:"systemMessage"`
	HookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

func parseSessionStart(t *testing.T, stdout string) sessionStart {
	t.Helper()
	var out sessionStart
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, stdout)
	}
	return out
}

// toolingOf is the <tooling> block alone, from its opening line to the
// closing tag.
func toolingOf(out string) string {
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

const fileSkillsYML = `global_skills: []
skill_triggers: {}
skill_file_map:
  - on: basename
    globs: ["*.go"]
    skills: [engineering-fundamentals]
  - on: basename
    globs: ["*.py"]
    skills: [python-patterns]
`

func TestInjectContext(t *testing.T) {
	t.Parallel()
	t.Run("SessionStart prunes expired state files and keeps fresh ones", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		old := filepath.Join(e.claude, "cache", cache.SkillsLoaded, "s0-bash-patterns")
		fresh := filepath.Join(e.claude, "cache", cache.SkillsLoaded, "s1-bash-patterns")
		testutil.Touch(t, old, fresh)
		testutil.Age(t, 48*time.Hour, old)
		e.inject("s1", "").Want(t, 0)
		if exists(old) || !exists(fresh) {
			t.Errorf("old kept %v, fresh kept %v; want only the fresh one", exists(old), exists(fresh))
		}
	})
	t.Run("SessionStart prunes the repo's gate restore points older than a month", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		tree := gitOut(t, e.root, "write-tree")
		old := "refs/kit/gates/" + strconv.FormatInt(time.Now().Add(-31*24*time.Hour).UnixNano(), 10)
		fresh := "refs/kit/gates/" + strconv.FormatInt(time.Now().Add(-29*24*time.Hour).UnixNano(), 10)
		testutil.Git(t, e.root, "update-ref", old, tree)
		testutil.Git(t, e.root, "update-ref", fresh, tree)
		e.inject("s1", "").Want(t, 0)
		if refs := gitOut(t, e.root, "for-each-ref", "--format=%(refname)", "refs/kit/gates/"); refs != fresh {
			t.Errorf("refs = %q, want only %s", refs, fresh)
		}
	})
	t.Run("<required-skills> contains every global_skills entry", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		e.kitYML("global_skills:\n  - fix-sizing\n  - context-gathering\nskill_triggers: {}\nstacks: {}\n")
		e.writeCache("root: "+e.root, "js: yes")
		r := e.inject("s1", "")
		r.Want(t, 0)
		r.Has(t, "<required-skills>", "fix-sizing", "context-gathering")
	})
	t.Run("<suggested-skills> has one line per skill a declared dependency maps to, with its trigger phrase", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		e.kitYML(`global_skills:
  - fix-sizing
skill_triggers:
  react-patterns: "before building or restructuring React components"
dependency_skills:
  - {deps: [react], skills: [react-patterns, fix-sizing]}
  - {deps: [vue], skills: [vue-patterns]}
`)
		testutil.Write(t, filepath.Join(e.root, "package.json"), `{"dependencies":{"react":"19.0.0"}}`+"\n")
		r := e.inject("s1", "")
		r.Want(t, 0)
		r.Has(t, "<suggested-skills>\nbefore building or restructuring React components: load react-patterns via the Skill tool\n</suggested-skills>")
	})
	t.Run("a suggested skill logs a suggested-skill marker to skills.jsonl", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		e.kitYML(`global_skills: []
skill_triggers:
  react-patterns: "before building or restructuring React components"
dependency_skills:
  - {deps: [react], skills: [react-patterns]}
`)
		testutil.Write(t, filepath.Join(e.root, "package.json"), `{"dependencies":{"react":"19.0.0"}}`+"\n")
		e.inject("s1", "").Want(t, 0)
		log := e.paths.LogFile(kitlog.Skills)
		if !exists(log) {
			t.Fatalf("%s missing", log)
		}
		n := 0
		for _, m := range testutil.JSONLines(t, log) {
			if m["event"] == "suggested-skill" && m["session_id"] == "s1" && m["skill_file"] == "react-patterns" {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%d suggested-skill markers, want 1:\n%s", n, read(t, log))
		}
	})
	t.Run("a subagent reuses its session's file scan; another session rescans", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		e.kitYML(fileSkillsYML)
		e.writeCache("root: "+e.root, "go: yes")
		testutil.Write(t, filepath.Join(e.root, "main.go"), "package main\n")
		r := e.inject("s1", "")
		r.Want(t, 0)
		r.Has(t, "load engineering-fundamentals via the Skill tool")

		testutil.Write(t, filepath.Join(e.root, "late.py"), "")
		agent := func(session string) result {
			return e.run("inject-subagent-context", map[string]any{"session_id": session, "cwd": e.root})
		}
		cached := agent("s1")
		cached.Want(t, 0)
		cached.Has(t, "load engineering-fundamentals via the Skill tool")
		cached.Lacks(t, "python-patterns")
		agent("s2").Has(t, "load python-patterns via the Skill tool")
	})
	t.Run("a kit.yml edit after the scan makes the session rescan", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		e.kitYML("global_skills: []\nskill_triggers: {}\n")
		e.writeCache("root: "+e.root, "go: yes")
		testutil.Write(t, filepath.Join(e.root, "tool.py"), "")
		e.inject("s1", "").Lacks(t, "python-patterns")

		e.kitYML(fileSkillsYML)
		e.writeCache("root: "+e.root, "go: yes")
		testutil.Age(t, -5*time.Second, e.paths.Base, e.stackCache("testproject", e.root))
		e.inject("s1", "").Has(t, "load python-patterns via the Skill tool")
	})
	t.Run("outside a git work tree, a directory walk finds the files", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		e.kitYML(fileSkillsYML + "skip_dirs: [node_modules, dist]\n")
		e.writeCache("root: "+e.root, "go: yes")
		if err := os.RemoveAll(filepath.Join(e.root, ".git")); err != nil {
			t.Fatal(err)
		}
		testutil.Write(t, filepath.Join(e.root, "go.mod"), "module x\n")
		testutil.Write(t, filepath.Join(e.root, "cmd", "main.go"), "package main\n")
		testutil.Write(t, filepath.Join(e.root, "node_modules", "dep", "x.py"), "")
		testutil.Write(t, filepath.Join(e.root, "dist", "gen.py"), "")
		r := e.inject("s1", "")
		r.Want(t, 0)
		r.Has(t, "load engineering-fundamentals via the Skill tool")
		r.Lacks(t, "python-patterns")
	})
	// A non-git project root reports its dirty-file count as unknown rather
	// than dropping the whole context.
	t.Run("a non-git project root degrades to dirty-files: unknown instead of failing open", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		nonGit := filepath.Join(e.tmp, "non-git-project")
		testutil.Mkdir(t, nonGit)
		e.kitYML("global_skills:\n  - fix-sizing\n")
		testutil.Write(t, e.stackCache("non-git-project", nonGit), "root: "+nonGit+"\njs: yes\n")
		r := e.inject("s1", nonGit)
		r.Want(t, 0)
		r.Has(t, "<required-skills>", "fix-sizing", "dirty-files (at session start): unknown")
	})
	t.Run("repo-context names the scratch dir without creating it", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		e.writeCache("root: "+e.root, "js: yes")
		r := e.inject("s1", "")
		r.Want(t, 0)
		r.Has(t, "scratch: "+e.root+"/.claude/scratch")
		if exists(filepath.Join(e.root, ".claude", "scratch")) {
			t.Error("scratch dir was created")
		}
	})

	// Install problems.
	t.Run("prereqs: a missing kit.yml gets a warning naming the plugin fix", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		if err := os.Remove(e.paths.Base); err != nil {
			t.Fatal(err)
		}
		r := e.inject("s1", "")
		r.Want(t, 0)
		r.Has(t, "readable kit.yml", "reinstall the plugin")
	})
	t.Run("a kit.yml that fails to load is reported, since every guard then runs with no config", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		e.kitYML("protected_branches: main\n")
		e.overlay("global_skills: [fix-sizing]\n")
		r := e.inject("s1", "")
		r.Want(t, 0)
		r.Has(t, "systemMessage", "kit.yml", "every guard runs with no config")
		msg := parseSessionStart(t, r.Stdout).SystemMessage
		if n := strings.Count(msg, "cannot unmarshal"); n != 1 {
			t.Errorf("the decode error appears %d times, want once:\n%s", n, msg)
		}
		if strings.Contains(msg, "claude-kit.local.yml") {
			t.Errorf("the valid overlay is blamed:\n%s", msg)
		}
	})
	t.Run("an overlay that breaks the merge is reported to the user and to Claude, with the context intact", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		e.kitYML("global_skills:\n  - fix-sizing\nprotected_branches: [main]\n")
		e.overlay("protected_branches: main\n")
		r := e.inject("s1", "")
		r.Want(t, 0)
		out := parseSessionStart(t, r.Stdout)
		if !strings.Contains(out.SystemMessage, "claude-kit.local.yml") || !strings.Contains(out.SystemMessage, "ignored") {
			t.Errorf("systemMessage = %q", out.SystemMessage)
		}
		ctx := out.HookSpecificOutput.AdditionalContext
		if out.HookSpecificOutput.HookEventName != "SessionStart" || !strings.Contains(ctx, "ignored") || !strings.Contains(ctx, "<required-skills>") {
			t.Errorf("hookSpecificOutput = %+v", out.HookSpecificOutput)
		}
	})

	// A rules dir left by install-rules.sh would load the rules twice.
	link := func(t *testing.T, e *contextEnv, target, name string) {
		if err := os.Symlink(target, filepath.Join(e.claude, "rules", name)); err != nil {
			t.Fatal(err)
		}
	}
	// partialRules is a copy of the kit's rules missing workflow.md.
	partialRules := func(t *testing.T, e *contextEnv) string {
		partial := filepath.Join(e.tmp, "partial-rules")
		copyRules(t, partial)
		if err := os.Remove(filepath.Join(partial, "workflow.md")); err != nil {
			t.Fatal(err)
		}
		return partial
	}
	t.Run("a leftover install-rules.sh link is reported, since the rules would load twice", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		clone := filepath.Join(e.tmp, "clone-rules")
		copyRules(t, clone)
		link(t, e, clone, "claude-kit")
		r := e.inject("s1", "")
		r.Want(t, 0)
		r.Has(t, "systemMessage", filepath.Join(e.claude, "rules", "claude-kit"), "load twice")
	})
	t.Run("a claude-kit link to an older clone missing a rule file is still reported", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		link(t, e, partialRules(t, e), "claude-kit")
		e.inject("s1", "").Has(t, "load twice")
	})
	t.Run("a claude-kit dir holding none of the kit's rule files is not the kit's", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		testutil.Write(t, filepath.Join(e.claude, "rules", "claude-kit", "my-notes.md"), "mine\n")
		e.inject("s1", "").Lacks(t, "load twice")
	})
	t.Run("an unrelated rules dir missing one of the kit's rule files is not the kit's", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		link(t, e, partialRules(t, e), "mine")
		e.inject("s1", "").Lacks(t, "load twice")
	})

	t.Run("tooling: a project stating no check gets the discovery instruction and the tools", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		e.useRealKitYML()
		r := e.inject("s1", "")
		r.Want(t, 0)
		block := toolingOf(r.Output)
		if !strings.Contains(block, "\ndiscovery: nothing in this project states a check") || !strings.Contains(block, "\navailable: ") || strings.Contains(block, "guidance:") {
			t.Errorf("tooling:\n%s", block)
		}
	})
	t.Run("tooling: tools are split into available and missing by PATH", func(t *testing.T) {
		t.Parallel()
		e := contextSetup(t)
		e.kitYML("tools: [jq, definitely-not-a-real-tool-xyz, git]\n")
		r := e.inject("s1", "")
		r.Want(t, 0)
		block := toolingOf(r.Output)
		for _, want := range []string{"available: jq, git", "missing: definitely-not-a-real-tool-xyz"} {
			if !strings.Contains(block, want) {
				t.Errorf("tooling lacks %q:\n%s", want, block)
			}
		}
	})
}

// Not parallel: these cases set the environment inject-context reads.
func TestInjectContextEnv(t *testing.T) {
	t.Run("<suggested-skills> adds the file-map skills a repo file matches, guard-skills on or off", func(t *testing.T) {
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
`
		for _, guard := range []string{"1", "0"} {
			e := contextSetup(t)
			e.kitYML(yml)
			testutil.Write(t, filepath.Join(e.root, "main.go"), "package main\n")
			testutil.Write(t, filepath.Join(e.root, "web", "app.test.js"), "")
			testutil.Write(t, filepath.Join(e.root, ".gitignore"), "ignored.py\n")
			testutil.Write(t, filepath.Join(e.root, "ignored.py"), "")
			t.Setenv("CLAUDE_GUARD_SKILLS", guard)
			r := e.inject("s1", "")
			r.Want(t, 0)
			r.Has(t, "load engineering-fundamentals via the Skill tool", "load test-patterns via the Skill tool")
			r.Lacks(t, "typescript-patterns", "python-patterns")
		}
	})
	// project.Name calls $HOME "home", a non-project context.
	t.Run("a non-project context (home) produces no injection", func(t *testing.T) {
		e := contextSetup(t)
		t.Setenv("HOME", e.home)
		e.kitYML("global_skills:\n  - fix-sizing\n")
		r := e.inject("s1", e.home)
		r.Want(t, 0)
		r.Empty(t)
	})
}
