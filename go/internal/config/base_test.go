package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// kitDir is a kit root holding kit.yml and the kit.d/ topic files named in
// topics, file name to body.
func kitDir(t *testing.T, base string, topics map[string]string) Paths {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "kit.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range topics {
		write(t, filepath.Join(dir, "kit.d"), name, body)
	}
	return Paths{Base: write(t, dir, "kit.yml", base), Overlay: filepath.Join(dir, "none.yml")}
}

func TestKitDTopicFilesMergeIntoKitYML(t *testing.T) {
	p := kitDir(t, "tools: [rg]\n", map[string]string{"guards.yml": "protected_branches: [main]\n", "skills.yml": "global_skills: [kit-rules]\n"})
	cfg, warnings, err := Load(p)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	if !slices.Equal(cfg.Tools, []string{"rg"}) || !slices.Equal(cfg.ProtectedBranches, []string{"main"}) || !slices.Equal(cfg.GlobalSkills, []string{"kit-rules"}) {
		t.Errorf("tools %q, protected_branches %q, global_skills %q", cfg.Tools, cfg.ProtectedBranches, cfg.GlobalSkills)
	}
}

func TestMergedNamesEachKeysKitDFile(t *testing.T) {
	p := kitDir(t, "tools: [rg]\n", map[string]string{"guards.yml": "protected_branches: [main]\n"})
	doc, err := Merged(p)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		got[doc.Content[i].Value] = doc.Content[i].LineComment + doc.Content[i+1].LineComment
	}
	if got["tools"] != "kit.yml" || got["protected_branches"] != "kit.d/guards.yml" {
		t.Errorf("sources %q", got)
	}
}

func TestAKeyInTwoBaseFilesWarnsAndMerges(t *testing.T) {
	p := kitDir(t, "tools: [rg]\n", map[string]string{"guards.yml": "tools: [jq]\n"})
	cfg, warnings, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || warnings[0].File != filepath.Join(filepath.Dir(p.Base), "kit.d", "guards.yml") || !strings.Contains(warnings[0].Err.Error(), "tools: kit.yml sets it too") {
		t.Errorf("warnings %v", warnings)
	}
	if !slices.Equal(cfg.Tools, []string{"rg", "jq"}) {
		t.Errorf("tools %q, want both files' entries", cfg.Tools)
	}
}

// A kit.d/ file written, edited, or removed is seen on the next cached load.
func TestLoadCacheFollowsKitD(t *testing.T) {
	p := kitDir(t, "protected_branches: [main]\n", nil)
	p.ClaudeDir = filepath.Dir(p.Base)
	topic := filepath.Join(p.BaseDir(), "guards.yml")
	branches := func() string {
		t.Helper()
		cfg, _, err := Load(p)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(cfg.ProtectedBranches, ",")
	}
	if got := branches(); got != "main" {
		t.Fatalf("first load = %s", got)
	}
	write(t, p.BaseDir(), "guards.yml", "rc_files: [x]\nprotected_branches: [dev]\n")
	if got := branches(); got != "main,dev" {
		t.Errorf("after a kit.d file appears = %s", got)
	}
	write(t, p.BaseDir(), "guards.yml", "protected_branches: [release]\n")
	if got := branches(); got != "main,release" {
		t.Errorf("after it changes = %s", got)
	}
	if err := os.Remove(topic); err != nil {
		t.Fatal(err)
	}
	if got := branches(); got != "main" {
		t.Errorf("after it goes = %s", got)
	}
}
