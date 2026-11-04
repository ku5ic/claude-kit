package chatbundle

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestLexiconMatchesTheGolden(t *testing.T) {
	t.Parallel()
	got := lexicon(config.Slop{
		BlockTiers: []int{1, 2},
		Density:    config.SlopDensity{PerWords: 100, Threshold: 3, Min: 3, Words: []string{"delve", "align with"}},
		Rules: []config.SlopRule{
			{ID: "ui-glass", Tier: 3, On: []string{"ui"}, Match: "backdrop-blur"},
			{ID: "arrow", Tier: 2, On: []string{"commit", "pr"}, Match: `\x{2192}`, Fix: "use ASCII"},
			{ID: "hope-helps", Tier: 1, Match: `(?i)\bI hope (this|that) helps\b`, Fix: "cut it"},
		},
	})
	if want := testutil.Read(t, "testdata/lexicon.golden"); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// kit.yml spells each tell character by code point, because the typography
// sanitizer would rewrite a literal one on its way into a file.
func TestRealLexiconIsASCII(t *testing.T) {
	t.Parallel()
	text := lexicon(testutil.KitConfig(t).Slop)
	for i, r := range text {
		if r > 127 {
			t.Fatalf("non-ASCII %q at byte %d", r, i)
		}
	}
	if !strings.Contains(text, "`hope-helps`") {
		t.Error("the real lexicon lost its rules")
	}
}

// scanCheckout is a checkout that bundles unslop.
func scanCheckout(t *testing.T) (string, *config.Config) {
	t.Helper()
	root, cfg := checkout(t)
	cfg.ChatBundle.Skills = []string{"alpha", "unslop"}
	testutil.Put(t, root, "skills/unslop/SKILL.md", "---\nname: unslop\ndescription: Find tells.\n---\n")
	testutil.Put(t, root, "templates/chat/scan.sh", "#!/bin/sh\n")
	testutil.Put(t, root, "kit.yml", "tools: [rg]\n")
	return root, cfg
}

func TestBundleCarriesTheScan(t *testing.T) {
	t.Parallel()
	root, cfg := scanCheckout(t)
	testutil.Put(t, root, "bin/kit-1.2.3-linux-amd64", "the binary")
	files := buildZip(t, root, cfg)
	for name, want := range map[string]string{
		"skills/unslop/scripts/kit-linux-amd64": "the binary",
		"skills/unslop/scripts/scan.sh":         "#!/bin/sh\n",
		"skills/unslop/kit.yml":                 "tools: [rg]\n",
	} {
		if files[name] != want {
			t.Errorf("%s: got %q, want %q", name, files[name], want)
		}
	}
	if !strings.HasPrefix(files["skills/unslop/reference/lexicon.md"], "# Lexicon") {
		t.Error("no lexicon")
	}
}

func TestOnlyABuildNeedsTheBinary(t *testing.T) {
	t.Parallel()
	root, cfg := scanCheckout(t)
	if code, _, stderr := run(root, cfg, "chat", "--check"); code != 0 {
		t.Errorf("--check: exit %d, stderr %s", code, stderr)
	}
	code, _, stderr := run(root, cfg, "chat")
	if code != 1 {
		t.Errorf("build: exit %d, want 1", code)
	}
	testutil.Has(t, stderr, "go/build.sh")
}

func TestBundleWithoutUnslopCarriesNoScan(t *testing.T) {
	t.Parallel()
	root, cfg := checkout(t)
	for name := range buildZip(t, root, cfg) {
		if strings.Contains(name, "unslop") {
			t.Errorf("unexpected %s", name)
		}
	}
}

func TestScanProblems(t *testing.T) {
	t.Parallel()
	for name, remove := range map[string]string{"no script": "templates/chat/scan.sh", "no kit.yml": "kit.yml"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, cfg := scanCheckout(t)
			if err := os.Remove(filepath.Join(root, remove)); err != nil {
				t.Fatal(err)
			}
			if code, _, _ := run(root, cfg, "chat", "--check"); code != 1 {
				t.Errorf("exit %d, want 1", code)
			}
		})
	}
}

func TestScanScript(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		kit        string // "" when the binary is missing
		wantExit   int
		wantStdout string
	}{
		"clean":                 {kit: "exit 0\n", wantExit: 0},
		"a hit":                 {kit: "echo hit\nexit 1\n", wantExit: 1, wantStdout: "hit\n"},
		"a note that passes":    {kit: "echo note\nexit 0\n", wantExit: 0, wantStdout: "note\n"},
		"silent failure":        {kit: "echo broken >&2\nexit 1\n", wantExit: 3},
		"usage error":           {kit: "exit 2\n", wantExit: 2},
		"no binary":             {wantExit: 3},
		"sees the skill folder": {kit: `[ "$CLAUDE_PLUGIN_ROOT" = "$(dirname "$(dirname "$SCAN")")" ] || exit 9` + "\necho \"$*\"\n", wantExit: 0, wantStdout: "slop --surface pr -\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := testutil.Physical(t, t.TempDir())
			script := filepath.Join(dir, "scripts", "scan.sh")
			testutil.Put(t, dir, "scripts/scan.sh", testutil.Read(t, "../../../templates/chat/scan.sh"))
			if tc.kit != "" {
				testutil.Stub(t, filepath.Join(dir, "scripts", "kit-linux-amd64"), tc.kit)
			}
			cmd := exec.Command("sh", script, "pr")
			cmd.Env = append(os.Environ(), "SCAN="+script)
			var stdout bytes.Buffer
			cmd.Stdout = &stdout
			code := 0
			if _, failed := errors.AsType[*exec.ExitError](cmd.Run()); failed {
				code = cmd.ProcessState.ExitCode()
			}
			if code != tc.wantExit {
				t.Errorf("exit %d, want %d", code, tc.wantExit)
			}
			if stdout.String() != tc.wantStdout {
				t.Errorf("stdout %q, want %q", &stdout, tc.wantStdout)
			}
		})
	}
}
