package chatbundle

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

const skillMD = "---\nname: alpha\ndescription: Use for alpha work.\n---\n\nBody.\n"

// checkout is a kit checkout with a bundled skill, alpha, and one the
// config leaves out.
func checkout(t *testing.T) (string, *config.Config) {
	t.Helper()
	root := t.TempDir()
	testutil.Put(t, root, ".claude-plugin/plugin.json", `{"name":"kit","version":"1.2.3","description":"The full kit.","author":{"name":"A"},"repository":"https://example.test/kit","license":"MIT"}`)
	testutil.Put(t, root, "skills/alpha/SKILL.md", skillMD)
	testutil.Put(t, root, "skills/alpha/reference/more.md", "More.\n")
	testutil.Put(t, root, "skills/beta/SKILL.md", "---\nname: beta\ndescription: Left out.\n---\n")
	testutil.Put(t, root, "skills/kit-rules/reference/change.md", "# Change\n\n## 1. One\n")
	testutil.Put(t, root, rulesHeadPath, "---\nname: kit-rules\ndescription: Load for coding.\n---\n\nThe head.\n")
	return root, &config.Config{ChatBundle: config.ChatBundle{Description: "For chat.", Skills: []string{"alpha"}, Rules: []string{"skills/kit-rules/reference/change.md"}}}
}

func run(root string, cfg *config.Config, args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = Run(root, cfg, args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// buildZip builds the bundle, which must succeed, and returns its files.
func buildZip(t *testing.T, root string, cfg *config.Config) map[string]string {
	t.Helper()
	if code, _, stderr := run(root, cfg, "chat"); code != 0 {
		t.Fatalf("exit %d, stderr %s", code, stderr)
	}
	return unzip(t, filepath.Join(root, "bin", "kit-chat-1.2.3.zip"))
}

func TestChatWritesTheNamedSkillsAndAChatManifest(t *testing.T) {
	t.Parallel()
	root, cfg := checkout(t)
	testutil.Put(t, root, "skills/alpha/.DS_Store", "junk")
	testutil.Put(t, root, "skills/alpha/.SKILL.md.swp", "junk")
	files := buildZip(t, root, cfg)
	_, stdout, _ := run(root, cfg, "chat")
	if got, want := strings.TrimSpace(stdout), filepath.Join(root, "bin", "kit-chat-1.2.3.zip"); got != want {
		t.Errorf("stdout %q, want the zip's path %q", got, want)
	}

	var names []string
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	want := []string{".claude-plugin/plugin.json", "skills/alpha/SKILL.md", "skills/alpha/reference/more.md", "skills/kit-rules/SKILL.md"}
	if !slices.Equal(names, want) {
		t.Errorf("entries %v, want %v", names, want)
	}

	var manifest map[string]any
	if err := json.Unmarshal([]byte(files[".claude-plugin/plugin.json"]), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest["name"] != "kit" || manifest["version"] != "1.2.3" || manifest["description"] != "For chat." || manifest["license"] != "MIT" {
		t.Errorf("manifest %v", manifest)
	}
}

func TestChatZipIsReproducible(t *testing.T) {
	t.Parallel()
	root, cfg := checkout(t)
	zipBytes := func() string {
		buildZip(t, root, cfg)
		return testutil.Read(t, filepath.Join(root, "bin", "kit-chat-1.2.3.zip"))
	}
	first := zipBytes()
	// A newer mtime on a source must not reach the archive.
	testutil.Put(t, root, "skills/alpha/SKILL.md", skillMD)
	if first != zipBytes() {
		t.Error("two builds of the same sources differ")
	}
}

func TestCheckValidatesAndWritesNothing(t *testing.T) {
	t.Parallel()
	root, cfg := checkout(t)
	code, stdout, stderr := run(root, cfg, "chat", "--check")
	if code != 0 {
		t.Fatalf("exit %d, stderr %s", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout %q, want nothing", stdout)
	}
	if _, err := os.Stat(filepath.Join(root, "bin")); err == nil {
		t.Error("--check wrote to bin")
	}
}

func TestBadBundlesFail(t *testing.T) {
	t.Parallel()
	type mutation = func(*testing.T, string, *config.Config)
	put := func(name, body string) mutation {
		return func(t *testing.T, root string, _ *config.Config) { testutil.Put(t, root, name, body) }
	}
	skill := func(front string) mutation { return put("skills/alpha/SKILL.md", "---\n"+front+"\n---\nBody.\n") }
	more := func(body string) mutation { return put("skills/alpha/reference/more.md", body) }
	bundling := func(c config.ChatBundle) mutation {
		return func(_ *testing.T, _ string, cfg *config.Config) { cfg.ChatBundle = c }
	}
	remove := func(name string) mutation {
		return func(t *testing.T, root string, _ *config.Config) {
			if err := os.RemoveAll(filepath.Join(root, name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	good := config.ChatBundle{Description: "For chat.", Skills: []string{"alpha"}, Rules: []string{"skills/kit-rules/reference/change.md"}}
	with := func(edit func(*config.ChatBundle)) config.ChatBundle {
		c := good
		edit(&c)
		return c
	}
	for name, tc := range map[string]struct {
		mutate mutation
		want   string
	}{
		"no frontmatter": {put("skills/alpha/SKILL.md", "Body.\n"), "no frontmatter"},
		"a longer dash line doesn't close frontmatter": {put("skills/alpha/SKILL.md", "---\nname: alpha\ndescription: x\n----\nBody.\n"), "no frontmatter"},
		"broken frontmatter":                           {skill("name: [alpha"), "frontmatter"},
		"no description":                               {skill("name: alpha"), "no description"},
		"name isn't the folder":                        {skill("name: other\ndescription: x"), `name "other"`},
		"long description":                             {skill("name: alpha\ndescription: " + strings.Repeat("x", 1025)), "1024"},
		"XML in a description":                         {skill("name: alpha\ndescription: Use for <example> work."), "XML"},
		"XML in a name":                                {skill("name: <alpha>\ndescription: x"), "XML"},
		"long plugin description":                      {bundling(with(func(c *config.ChatBundle) { c.Description = strings.Repeat("x", 501) })), "500"},
		"no plugin description":                        {bundling(with(func(c *config.ChatBundle) { c.Description = "" })), "chat_bundle.description"},
		"no skills":                                    {bundling(with(func(c *config.ChatBundle) { c.Skills = nil })), "chat_bundle.skills"},
		"missing skill":                                {bundling(with(func(c *config.ChatBundle) { c.Skills = []string{"gone"} })), "gone"},
		"no rules":                                     {bundling(with(func(c *config.ChatBundle) { c.Rules = nil })), "chat_bundle.rules"},
		"missing rule":                                 {bundling(with(func(c *config.ChatBundle) { c.Rules = []string{"skills/kit-rules/reference/gone.md"} })), "skills/kit-rules/reference/gone.md"},
		"no rules head":                                {remove(rulesHeadPath), "kit-rules.md"},
		"no skills directory":                          {remove("skills"), "skills"},
		"a rules skill too long":                       {put("skills/kit-rules/reference/change.md", "# Change\n"+strings.Repeat("line\n", maxSkillLines)), "skills/kit-rules/SKILL.md: 500 lines"},
		"rule with no title":                           {put("skills/kit-rules/reference/change.md", "text\n"), "skills/kit-rules/reference/change.md: no # title"},
		"unclosed marker in a rule":                    {put("skills/kit-rules/reference/change.md", "# Change\n<!-- chat: skip -->\n"), "skills/kit-rules/reference/change.md: line 2"},
		"unclosed marker in a skill":                   {more("<!-- chat: skip -->\n"), "skills/alpha/reference/more.md: line 1: marker never closed"},
		"lint hit":                                     {more("ok\nRun `kit scratch-dir`.\n"), "skills/alpha/reference/more.md:2: kit command"},
		"a left-out skill by name":                     {more("See `beta`.\n"), "skill not in the bundle"},
		"section a marker dropped": {func(t *testing.T, root string, cfg *config.Config) {
			put("skills/kit-rules/reference/change.md", "# Change\n\n<!-- chat: skip -->\n\n## 1. One\n\n<!-- chat: end -->\n")(t, root, cfg)
			more("`skills/kit-rules/reference/change.md` section 1\n")(t, root, cfg)
		}, "links skills/kit-rules/reference/change.md section 1"},
		"manifest isn't JSON":     {put(".claude-plugin/plugin.json", "{"), "plugin.json"},
		"manifest has no name":    {put(".claude-plugin/plugin.json", `{"version":"1.0.0"}`), "name"},
		"manifest has no version": {put(".claude-plugin/plugin.json", `{"name":"kit"}`), "version"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, cfg := checkout(t)
			tc.mutate(t, root, cfg)
			for _, args := range [][]string{{"chat"}, {"chat", "--check"}} {
				code, _, stderr := run(root, cfg, args...)
				if code != 1 {
					t.Errorf("%v: exit %d, want 1", args, code)
				}
				if !strings.Contains(stderr, tc.want) {
					t.Errorf("%v: stderr %q lacks %q", args, stderr, tc.want)
				}
			}
			if _, err := os.Stat(filepath.Join(root, "bin", "kit-chat-1.2.3.zip")); err == nil {
				t.Error("a failed build left a zip")
			}
		})
	}
}

func TestCRLFFrontmatterPasses(t *testing.T) {
	t.Parallel()
	root, cfg := checkout(t)
	testutil.Put(t, root, "skills/alpha/SKILL.md", "---\r\nname: alpha\r\ndescription: Use for alpha work.\r\n---\r\nBody.\r\n")
	if code, _, stderr := run(root, cfg, "chat", "--check"); code != 0 {
		t.Errorf("exit %d, stderr %s", code, stderr)
	}
}

func TestEveryProblemIsReported(t *testing.T) {
	t.Parallel()
	root, cfg := checkout(t)
	testutil.Put(t, root, "skills/alpha/SKILL.md", "---\nname: other\n---\n")
	cfg.ChatBundle.Description = strings.Repeat("x", 501)
	_, _, stderr := run(root, cfg, "chat")
	testutil.Has(t, stderr, "500", "no description", `name "other"`)
}

func TestUsageErrors(t *testing.T) {
	t.Parallel()
	for name, args := range map[string][]string{
		"no kind":       {},
		"unknown kind":  {"nope"},
		"unknown flag":  {"chat", "--nope"},
		"flag for kind": {"--check"},
		"extra arg":     {"chat", "more"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, cfg := checkout(t)
			code, _, stderr := run(root, cfg, args...)
			if code != 2 {
				t.Errorf("exit %d, want 2", code)
			}
			testutil.Has(t, stderr, "usage: kit bundle")
		})
	}
}

func TestHelpGoesToStdout(t *testing.T) {
	t.Parallel()
	root, cfg := checkout(t)
	code, stdout, _ := run(root, cfg, "--help")
	if code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
	testutil.Has(t, stdout, "usage: kit bundle")
}

func TestUnwritableOutputFails(t *testing.T) {
	t.Parallel()
	root, cfg := checkout(t)
	testutil.Put(t, root, "bin", "a file where the directory goes")
	code, _, stderr := run(root, cfg, "chat")
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	if stderr == "" {
		t.Error("no error reported")
	}
}

func unzip(t *testing.T, path string) map[string]string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	files := map[string]string{}
	for _, f := range r.File {
		if f.Method != zip.Deflate {
			t.Errorf("%s: method %d, want DEFLATE", f.Name, f.Method)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name] = string(data)
	}
	return files
}
