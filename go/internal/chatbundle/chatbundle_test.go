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
	if err := os.Mkdir(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root, &config.Config{ChatBundle: config.ChatBundle{Description: "For chat.", Skills: []string{"alpha"}}}
}

func TestChatWritesTheNamedSkillsAndAChatManifest(t *testing.T) {
	t.Parallel()
	root, cfg := checkout(t)
	testutil.Put(t, root, "skills/alpha/.DS_Store", "junk")
	testutil.Put(t, root, "skills/alpha/.SKILL.md.swp", "junk")
	var stdout, stderr bytes.Buffer
	if code := Run(root, cfg, []string{"chat"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr %s", code, &stderr)
	}
	out := filepath.Join(root, "bin", "kit-chat-1.2.3.zip")
	if got := strings.TrimSpace(stdout.String()); got != out {
		t.Errorf("stdout %q, want the zip's path %q", got, out)
	}

	files := unzip(t, out)
	var names []string
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	want := []string{".claude-plugin/plugin.json", "skills/alpha/SKILL.md", "skills/alpha/reference/more.md"}
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
	out := filepath.Join(root, "bin", "kit-chat-1.2.3.zip")
	build := func() []byte {
		var stderr bytes.Buffer
		if code := Run(root, cfg, []string{"chat"}, io.Discard, &stderr); code != 0 {
			t.Fatalf("exit %d, stderr %s", code, &stderr)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	first := build()
	// A newer mtime on a source must not reach the archive.
	testutil.Put(t, root, "skills/alpha/SKILL.md", skillMD)
	if !bytes.Equal(first, build()) {
		t.Error("two builds of the same sources differ")
	}
}

func TestCheckValidatesAndWritesNothing(t *testing.T) {
	t.Parallel()
	root, cfg := checkout(t)
	var stdout, stderr bytes.Buffer
	if code := Run(root, cfg, []string{"chat", "--check"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr %s", code, &stderr)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout %q, want nothing", &stdout)
	}
	if entries, _ := os.ReadDir(filepath.Join(root, "bin")); len(entries) != 0 {
		t.Errorf("bin holds %v, want nothing", entries)
	}
}

func TestUploadRulesFailTheBuild(t *testing.T) {
	t.Parallel()
	skill := func(front string) func(*testing.T, string, *config.Config) {
		return func(t *testing.T, root string, _ *config.Config) {
			testutil.Put(t, root, "skills/alpha/SKILL.md", "---\n"+front+"\n---\nBody.\n")
		}
	}
	for name, tc := range map[string]struct {
		mutate func(*testing.T, string, *config.Config)
		want   string
	}{
		"no frontmatter": {func(t *testing.T, root string, _ *config.Config) {
			testutil.Put(t, root, "skills/alpha/SKILL.md", "Body.\n")
		}, "no frontmatter"},
		"a longer dash line doesn't close frontmatter": {func(t *testing.T, root string, _ *config.Config) {
			testutil.Put(t, root, "skills/alpha/SKILL.md", "---\nname: alpha\ndescription: x\n----\nBody.\n")
		}, "no frontmatter"},
		"broken frontmatter":    {skill("name: [alpha"), "frontmatter"},
		"no description":        {skill("name: alpha"), "no description"},
		"name isn't the folder": {skill("name: other\ndescription: x"), `name "other"`},
		"long description":      {skill("name: alpha\ndescription: " + strings.Repeat("x", 1025)), "1024"},
		"XML in a description":  {skill("name: alpha\ndescription: Use for <example> work."), "XML"},
		"XML in a name":         {skill("name: <alpha>\ndescription: x"), "XML"},
		"long plugin description": {func(_ *testing.T, _ string, cfg *config.Config) {
			cfg.ChatBundle.Description = strings.Repeat("x", 501)
		}, "500"},
		"no plugin description": {func(_ *testing.T, _ string, cfg *config.Config) {
			cfg.ChatBundle.Description = ""
		}, "chat_bundle.description"},
		"no skills": {func(_ *testing.T, _ string, cfg *config.Config) {
			cfg.ChatBundle.Skills = nil
		}, "chat_bundle.skills"},
		"missing skill": {func(_ *testing.T, _ string, cfg *config.Config) {
			cfg.ChatBundle.Skills = []string{"gone"}
		}, "gone"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, cfg := checkout(t)
			tc.mutate(t, root, cfg)
			for _, args := range [][]string{{"chat"}, {"chat", "--check"}} {
				var stdout, stderr bytes.Buffer
				if code := Run(root, cfg, args, &stdout, &stderr); code != 1 {
					t.Errorf("%v: exit %d, want 1", args, code)
				}
				if !strings.Contains(stderr.String(), tc.want) {
					t.Errorf("%v: stderr %q lacks %q", args, &stderr, tc.want)
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
	var stdout, stderr bytes.Buffer
	if code := Run(root, cfg, []string{"chat", "--check"}, &stdout, &stderr); code != 0 {
		t.Errorf("exit %d, stderr %s", code, &stderr)
	}
}

func TestEveryProblemIsReported(t *testing.T) {
	t.Parallel()
	root, cfg := checkout(t)
	testutil.Put(t, root, "skills/alpha/SKILL.md", "---\nname: other\n---\n")
	cfg.ChatBundle.Description = strings.Repeat("x", 501)
	var stderr bytes.Buffer
	Run(root, cfg, []string{"chat"}, io.Discard, &stderr)
	for _, want := range []string{"500", "no description", `name "other"`} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr %q lacks %q", &stderr, want)
		}
	}
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
			var stderr bytes.Buffer
			if code := Run(root, cfg, args, io.Discard, &stderr); code != 2 {
				t.Errorf("exit %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), "usage: kit bundle") {
				t.Errorf("stderr %q has no usage", &stderr)
			}
		})
	}
}

func TestHelpGoesToStdout(t *testing.T) {
	t.Parallel()
	root, cfg := checkout(t)
	var stdout bytes.Buffer
	if code := Run(root, cfg, []string{"--help"}, &stdout, io.Discard); code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "usage: kit bundle") {
		t.Errorf("stdout %q has no usage", &stdout)
	}
}

func TestManifestProblems(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ manifest, want string }{
		"not JSON":   {"{", "plugin.json"},
		"no name":    {`{"version":"1.0.0"}`, "name"},
		"no version": {`{"name":"kit"}`, "version"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, cfg := checkout(t)
			testutil.Put(t, root, ".claude-plugin/plugin.json", tc.manifest)
			var stderr bytes.Buffer
			if code := Run(root, cfg, []string{"chat"}, io.Discard, &stderr); code != 1 {
				t.Errorf("exit %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Errorf("stderr %q lacks %q", &stderr, tc.want)
			}
		})
	}
}

func TestUnwritableOutputFails(t *testing.T) {
	t.Parallel()
	root, cfg := checkout(t)
	if err := os.Remove(filepath.Join(root, "bin")); err != nil {
		t.Fatal(err)
	}
	testutil.Put(t, root, "bin", "a file where the directory goes")
	var stderr bytes.Buffer
	if code := Run(root, cfg, []string{"chat"}, io.Discard, &stderr); code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	if stderr.Len() == 0 {
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
