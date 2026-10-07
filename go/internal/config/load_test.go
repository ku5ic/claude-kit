package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRealKitYMLLoadsCleanly(t *testing.T) {
	// The personal overlay, when CLAUDE_KIT_PERSONAL points at one.
	var overlay string
	if dir := os.Getenv("CLAUDE_KIT_PERSONAL"); dir != "" {
		overlay = filepath.Join(dir, "claude-kit.local.yml")
	}
	_, statErr := os.Stat(overlay)
	cfg, warnings, err := Load(Paths{Base: "../../../kit.yml", Overlay: overlay})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range warnings {
		t.Errorf("warning: %s", w)
	}
	lists := map[string]int{
		"package_managers":   len(cfg.PackageManagers),
		"protected_branches": len(cfg.ProtectedBranches),
		"rc_files":           len(cfg.RCFiles),
		"sensitive_paths":    len(cfg.SensitivePaths),
		"task_providers":     len(cfg.TaskProviders),
		"checks":             len(cfg.Checks),
		"toolchain_checks":   len(cfg.ToolchainChecks),
		"orchestrators":      len(cfg.Orchestrators),
		"tools":              len(cfg.Tools),
		"formatters":         len(cfg.Formatters),
		"stacks":             len(cfg.Stacks),
	}
	for name, n := range lists {
		if n == 0 {
			t.Errorf("%s is empty", name)
		}
	}
	if _, ok := cfg.Stacks["dotfiles"]; statErr == nil && !ok {
		t.Error("overlay stack dotfiles not merged")
	}
}

func TestOverlayMergesMapsAndAppendsSequences(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "kit.yml", "protected_branches: [main]\nlog_max_lines: 5\nstacks:\n  js:\n    skills: [a]\n")
	overlay := write(t, dir, "over.yml", "protected_branches: [develop]\nlog_max_lines: 7\nstacks:\n  js:\n    skills: [b]\n  go:\n    skills: [c]\n")
	cfg, warnings, err := Load(Paths{Base: base, Overlay: overlay})
	if err != nil || len(warnings) > 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	if got := strings.Join(cfg.ProtectedBranches, ","); got != "main,develop" {
		t.Errorf("protected_branches = %s", got)
	}
	if cfg.LogMaxLines != 7 {
		t.Errorf("log_max_lines = %d, want the overlay's 7", cfg.LogMaxLines)
	}
	if got := strings.Join(cfg.Stacks["js"].Skills, ","); got != "a,b" {
		t.Errorf("js skills = %s", got)
	}
	if got := strings.Join(cfg.Stacks["go"].Skills, ","); got != "c" {
		t.Errorf("go skills = %s", got)
	}
}

func TestOverlayFormatterWithADefaultsNameUpdatesItFieldByField(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "kit.yml", "formatters:\n  - {name: a, ext: [md], bin: a, cmd: \"{bin} {file}\"}\n  - {name: b, ext: [py], bin: b}\n")
	overlay := write(t, dir, "over.yml", "formatters:\n  - {name: a, ext: [mdx]}\n  - {name: c, ext: [toml], bin: c}\n")
	cfg, warnings, err := Load(Paths{Base: base, Overlay: overlay})
	if err != nil || len(warnings) > 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	var got []string
	for _, f := range cfg.Formatters {
		got = append(got, f.Name+":"+strings.Join(f.Ext, ",")+":"+f.Bin+":"+f.Cmd)
	}
	if want := "a:mdx:a:{bin} {file}|b:py:b:|c:toml:c:"; strings.Join(got, "|") != want {
		t.Errorf("formatters = %s, want %s", strings.Join(got, "|"), want)
	}
}

func TestOverlayChecksAndToolchainChecksUpdateByKey(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "kit.yml", "checks:\n  - {name: test, tasks: [test], exclude: [\"*watch*\"]}\n"+
		"toolchain_checks:\n  - {stack: go, name: test, cmd: \"{bin} test ./...\", bin: [go]}\n  - {stack: rust, name: test, cmd: \"{bin} test\", bin: [cargo]}\n")
	overlay := write(t, dir, "over.yml", "checks:\n  - {name: test, tasks: [test, \"test:unit\"]}\n"+
		"toolchain_checks:\n  - {stack: go, name: test, cmd: \"{bin} test -race ./...\"}\n")
	cfg, warnings, err := Load(Paths{Base: base, Overlay: overlay})
	if err != nil || len(warnings) > 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	if len(cfg.Checks) != 1 || strings.Join(cfg.Checks[0].Tasks, ",") != "test,test:unit" || strings.Join(cfg.Checks[0].Exclude, ",") != "*watch*" {
		t.Errorf("checks = %+v", cfg.Checks)
	}
	var got []string
	for _, tc := range cfg.ToolchainChecks {
		got = append(got, tc.Stack+":"+tc.Cmd+":"+strings.Join(tc.Bin, ","))
	}
	if want := "go:{bin} test -race ./...:go|rust:{bin} test:cargo"; strings.Join(got, "|") != want {
		t.Errorf("toolchain_checks = %s, want %s", strings.Join(got, "|"), want)
	}
}

func TestCheckDisabledMatchesSlotOrLabel(t *testing.T) {
	cfg := &Config{DisabledChecks: []string{"typecheck", "lint (lint:css) [web]", "js: test (test:unit)"}}
	for _, c := range []struct {
		slot, label string
		want        bool
	}{
		{"typecheck", "js: typecheck (tsc)", true},
		{"lint", "js: lint (lint:css) [web]", true},
		{"lint", "js: lint (lint) [web]", false},
		{"test", "js: test (test:unit)", true},
		{"test", "js: test (test)", false},
	} {
		if got := cfg.CheckDisabled(c.slot, c.label); got != c.want {
			t.Errorf("CheckDisabled(%q, %q) = %v, want %v", c.slot, c.label, got, c.want)
		}
	}
}

func TestUnknownKeysWarnWithTheirFile(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "kit.yml", "protected_branches: [main]\nformatters:\n  - name: x\n    signal_fies: [a]\n")
	overlay := write(t, dir, "over.yml", "protected_brnches: [dev]\n")
	cfg, warnings, err := Load(Paths{Base: base, Overlay: overlay})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 2 {
		t.Fatalf("want 2 warnings, got %v", warnings)
	}
	if warnings[0].File != base || !strings.Contains(warnings[0].Err.Error(), "signal_fies") {
		t.Errorf("first warning = %s", warnings[0])
	}
	if warnings[1].File != overlay || !strings.Contains(warnings[1].Err.Error(), "protected_brnches") {
		t.Errorf("second warning = %s", warnings[1])
	}
	if strings.Join(cfg.ProtectedBranches, ",") != "main" {
		t.Errorf("a misspelled overlay key must not change anything: %v", cfg.ProtectedBranches)
	}
}

func TestDefaultsAndMissingOverlay(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "kit.yml", "tools: [rg]\n")
	cfg, warnings, err := Load(Paths{Base: base, Overlay: filepath.Join(dir, "absent.yml")})
	if err != nil || len(warnings) > 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	if cfg.LogMaxLines != 10000 || cfg.SubprojectMaxDepth != 4 {
		t.Errorf("defaults: log_max_lines=%d subproject_max_depth=%d", cfg.LogMaxLines, cfg.SubprojectMaxDepth)
	}
}

func TestEmptyOrCommentOnlyOverlayIsQuiet(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "kit.yml", "tools: [rg]\n")
	for _, body := range []string{"", "# nothing yet\n"} {
		overlay := write(t, dir, "over.yml", body)
		if _, warnings, err := Load(Paths{Base: base, Overlay: overlay}); err != nil || len(warnings) > 0 {
			t.Errorf("overlay %q: err=%v warnings=%v", body, err, warnings)
		}
	}
}

// A type error only surfaces after the merge (a scalar replaces the base's
// list), and used to nil the whole config, switching every guard off.
func TestOverlayThatBreaksTheMergeFallsBackToBase(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "kit.yml", "protected_branches: [main]\n")
	overlay := write(t, dir, "over.yml", "protected_branches: main\n")
	cfg, warnings, err := Load(Paths{Base: base, Overlay: overlay})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(cfg.ProtectedBranches, ",") != "main" {
		t.Errorf("protected_branches = %v", cfg.ProtectedBranches)
	}
	if cfg.Tag != "base" {
		t.Errorf("tag = %q, want base", cfg.Tag)
	}
	ignored := false
	for _, w := range warnings {
		ignored = ignored || (w.File == overlay && strings.HasPrefix(w.Err.Error(), "ignored"))
	}
	if !ignored {
		t.Errorf("no ignored-overlay warning in %v", warnings)
	}
}

func TestBrokenOverlayIsIgnoredWithAWarning(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "kit.yml", "protected_branches: [main]\n")
	overlay := write(t, dir, "over.yml", "protected_branches: [unclosed\n")
	cfg, warnings, err := Load(Paths{Base: base, Overlay: overlay})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || warnings[0].File != overlay {
		t.Fatalf("warnings = %v", warnings)
	}
	if strings.Join(cfg.ProtectedBranches, ",") != "main" {
		t.Errorf("protected_branches = %v", cfg.ProtectedBranches)
	}
}
