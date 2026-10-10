package config

import (
	"os"
	"path/filepath"
	"regexp"
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
	if len(cfg.ToolResolution.EnvLookups) == 0 {
		t.Error("tool_resolution.env_lookups is empty")
	}
	for _, eco := range []string{"js", "python"} {
		if cfg.DefaultManager(eco) == "" {
			t.Errorf("no default manager for %s", eco)
		}
	}
	for _, name := range []string{"npx", "bunx", "pip3"} {
		if _, ok := cfg.Manager(name); !ok {
			t.Errorf("no package manager answers to %s", name)
		}
	}
	for _, pm := range cfg.PackageManagers {
		if _, err := regexp.Compile(pm.GlobalInstall); err != nil {
			t.Errorf("%s global_install: %v", pm.Manager, err)
		}
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
	cfg := &Config{DisabledChecks: []string{"typecheck", "lint (lint:css) [web]", "js: test (test:unit)", "js: off", "", "lint (.github/workflows/ci.yml: eslint)"}}
	for _, c := range []struct {
		slot, label string
		want        bool
	}{
		{"typecheck", "js: typecheck (tsc)", true},
		{"lint", "js: lint (lint:css) [web]", true},
		{"lint", "js: lint (lint) [web]", false},
		{"test", "js: test (test:unit)", true},
		{"test", "js: test (test)", false},
		// A label without " [dir]" names the check in every subproject.
		{"test", "js: test (test:unit) [e2e]", true},
		{"other", "js: off", true},
		{"other", "js: off [e2e]", true},
		{"other", "js: offline [e2e]", false},
		// A "[dir]" label stays exact.
		{"lint", "js: lint (lint:css) [api]", false},
		// An empty entry names nothing, not a check with no slot.
		{"", "go: vet [go]", false},
		{"lint", "js: lint (.github/workflows/ci.yml: eslint) [web]", true},
	} {
		if got := cfg.CheckDisabled(c.slot, c.label); got != c.want {
			t.Errorf("CheckDisabled(%q, %q) = %v, want %v", c.slot, c.label, got, c.want)
		}
	}
}

func TestDisablesThatMatchNothingWarn(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "kit.yml", "checks:\n  - {name: lint}\n"+
		"toolchain_checks:\n  - {stack: go, name: vet, cmd: x}\n")
	overlay := write(t, dir, "over.yml", "disabled_checks: [lint, \"js: lint (lint:css) [web]\", vet, bogus, \"js: lnt (x)\", \"lint (.github/workflows/ci.yml: eslint)\", \"\"]\n"+
		"disabled_toolchain_checks: [\"go:vet\", \"go:nope\", vet]\n")
	_, warnings, err := Load(Paths{Base: base, Overlay: overlay})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, w := range warnings {
		if w.File != overlay {
			t.Errorf("warning names %s, want the overlay", w.File)
		}
		got = append(got, w.Err.Error())
	}
	want := `disabled_checks: "bogus" names no check|disabled_checks: "js: lnt (x)" names no check|` +
		`disabled_checks: "" names no check|` +
		`disabled_toolchain_checks: "go:nope" names no toolchain check (<stack>:<name>)|` +
		`disabled_toolchain_checks: "vet" names no toolchain check (<stack>:<name>)`
	if strings.Join(got, "|") != want {
		t.Errorf("warnings =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.ReplaceAll(want, "|", "\n"))
	}
}

func TestNegativeReplyLimitsWarn(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "kit.yml", "reply_limits:\n  chat: -5\n  explain: 80\n  write: {commit: -1}\n")
	_, warnings, err := Load(Paths{Base: base})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, w := range warnings {
		got = append(got, w.Err.Error())
	}
	want := `reply_limits: chat is -5; a ceiling is 0 (off) or more|reply_limits: write.commit is -1; a ceiling is 0 (off) or more`
	if strings.Join(got, "|") != want {
		t.Errorf("warnings = %q, want %q", got, want)
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

// With a Home, Load serves its cached result until a file changes: a
// cached load matches a fresh one, warnings included, and an overlay
// written, edited, or removed is seen on the next load.
func TestLoadCacheFollowsTheFiles(t *testing.T) {
	dir := t.TempDir()
	p := Paths{Home: dir, Base: write(t, dir, "kit.yml", "protected_branches: [main]\n"), Overlay: filepath.Join(dir, "over.yml")}
	branches := func(wantWarnings int) string {
		t.Helper()
		cfg, warnings, err := Load(p)
		if err != nil || len(warnings) != wantWarnings {
			t.Fatalf("err=%v warnings=%v, want %d", err, warnings, wantWarnings)
		}
		return strings.Join(cfg.ProtectedBranches, ",")
	}
	if got := branches(0); got != "main" {
		t.Fatalf("first load = %s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "cache", "config.gob")); err != nil {
		t.Fatalf("no cache written: %v", err)
	}
	if got := branches(0); got != "main" {
		t.Errorf("cached load = %s", got)
	}
	write(t, dir, "over.yml", "protected_branches: [dev]\nprotected_brnches: [x]\n")
	for range 2 { // fresh, then cached
		if got := branches(1); got != "main,dev" {
			t.Errorf("after the overlay appears = %s", got)
		}
	}
	write(t, dir, "over.yml", "protected_branches: [release]\n")
	if got := branches(0); got != "main,release" {
		t.Errorf("after the overlay changes = %s", got)
	}
	if err := os.Remove(p.Overlay); err != nil {
		t.Fatal(err)
	}
	if got := branches(0); got != "main" {
		t.Errorf("after the overlay goes = %s", got)
	}
}

func TestDefaultsAndMissingOverlay(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "kit.yml", "tools: [rg]\n")
	cfg, warnings, err := Load(Paths{Base: base, Overlay: filepath.Join(dir, "absent.yml")})
	if err != nil || len(warnings) > 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	if cfg.LogMaxLines != 10000 || cfg.SubprojectMaxDepth != 4 || cfg.CheckTimeout != 90 {
		t.Errorf("defaults: log_max_lines=%d subproject_max_depth=%d check_timeout=%d", cfg.LogMaxLines, cfg.SubprojectMaxDepth, cfg.CheckTimeout)
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

func TestSessionFile(t *testing.T) {
	p := Paths{Home: "/h"}
	if got := p.SessionFile("skills-loaded", "s1", "bash-patterns"); got != "/h/cache/skills-loaded/s1-bash-patterns" {
		t.Errorf("got %q", got)
	}
	if got := p.SessionFile("plan-active", "s1"); got != "/h/cache/plan-active/s1" {
		t.Errorf("got %q", got)
	}
	for _, bad := range []string{"", "../x", `a\b`} {
		if got := p.SessionFile("plan-active", bad); got != "" {
			t.Errorf("session %q: %q, want none", bad, got)
		}
	}
}
