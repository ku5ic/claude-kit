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
	cfg, warnings, err := Load(Paths{Base: "../../../kit.yml", Overlay: overlay})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range warnings {
		t.Errorf("warning: %s", w)
	}
	lists := map[string]int{
		"protected_branches": len(cfg.ProtectedBranches),
		"rc_files":           len(cfg.RCFiles),
		"sensitive_paths":    len(cfg.SensitivePaths),
		"tools":              len(cfg.Tools),
		"on_edit":            len(cfg.OnEdit),
		"dependency_skills":  len(cfg.DependencySkills),
		"lockfile_globs":     len(cfg.LockfileGlobs),
		"deny_flags":         len(cfg.DenyFlags),
		"gate_env":           len(cfg.GateEnv),
		"global_installs":    len(cfg.GlobalInstalls),
	}
	for name, n := range lists {
		if n == 0 {
			t.Errorf("%s is empty", name)
		}
	}
	for _, pattern := range cfg.GlobalInstalls {
		if _, err := regexp.Compile(pattern); err != nil {
			t.Errorf("global_installs %q: %v", pattern, err)
		}
	}
}

func TestOverlayMergesMapsAndAppendsSequences(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "kit.yml", "protected_branches: [main]\nlog_max_lines: 5\nreply_limits:\n  chat: 40\n  write: {commit: 50}\n")
	overlay := write(t, dir, "over.yml", "protected_branches: [develop]\nlog_max_lines: 7\nreply_limits:\n  write: {pr: 80}\n")
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
	if l := cfg.ReplyLimits; l.Chat != 40 || l.Write["commit"] != 50 || l.Write["pr"] != 80 {
		t.Errorf("reply_limits = %+v", l)
	}
}

// The overlay's on_edit rules come after kit.yml's, so the last match, the
// overlay's, wins.
func TestOverlayOnEditRulesAppend(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "kit.yml", "on_edit:\n  - {globs: [\"*.md\"], run: [{cmd: \"a {file}\"}]}\n")
	overlay := write(t, dir, "over.yml", "on_edit:\n  - {globs: [\"*.toml\"], run: [{cmd: \"c {file}\", stdout: true}]}\n")
	cfg, warnings, err := Load(Paths{Base: base, Overlay: overlay})
	if err != nil || len(warnings) > 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	if len(cfg.OnEdit) != 2 || cfg.OnEdit[1].Globs[0] != "*.toml" || !cfg.OnEdit[1].Run[0].Stdout {
		t.Errorf("on_edit = %+v", cfg.OnEdit)
	}
}

func TestCheckDisabledMatchesKindOrLabel(t *testing.T) {
	cfg := &Config{DisabledChecks: []string{"typecheck", "lint (package.json: lint:css) [web]", "test (package.json: test:unit)", ""}}
	for _, c := range []struct {
		kind, label string
		want        bool
	}{
		{"typecheck", "typecheck (package.json: tsc)", true},
		{"lint", "lint (package.json: lint:css) [web]", true},
		{"lint", "lint (package.json: lint) [web]", false},
		{"test", "test (package.json: test:unit)", true},
		{"test", "test (package.json: test)", false},
		// A label without " [dir]" names the gate in every subproject.
		{"test", "test (package.json: test:unit) [e2e]", true},
		// A "[dir]" label stays exact.
		{"lint", "lint (package.json: lint:css) [api]", false},
		// An empty entry names nothing, not a gate with no kind.
		{"", "check (Makefile: all)", false},
	} {
		if got := cfg.CheckDisabled(c.kind, c.label); got != c.want {
			t.Errorf("CheckDisabled(%q, %q) = %v, want %v", c.kind, c.label, got, c.want)
		}
	}
}

// A single word that isn't a kind names nothing; a label can't be judged
// without the project.
func TestDisablesThatCanNameNothingWarn(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "kit.yml", "")
	overlay := write(t, dir, "over.yml", "disabled_checks: [lint, \"lint (package.json: lint:css) [web]\", vet, \"\"]\n")
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
	kinds := "lint, typecheck, test, format-check, deadcode, security"
	want := `disabled_checks: "vet" is no check kind (` + kinds + `) or gate label|disabled_checks: "" is no check kind (` + kinds + `) or gate label`
	if strings.Join(got, "|") != want {
		t.Errorf("warnings =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.ReplaceAll(want, "|", "\n"))
	}
}

func TestDeniedFlag(t *testing.T) {
	cfg := &Config{DenyFlags: []string{"--fix", "--watch"}}
	for words, want := range map[string]string{
		"eslint --fix .":                 "--fix",
		"vitest --watch=true":            "--watch",
		"vitest run --watch=false":       "",
		"eslint --fix-dry-run .":         "",
		"prettier --check . --watch=off": "",
	} {
		if got := cfg.DeniedFlag(strings.Fields(words)); got != want {
			t.Errorf("%q: %q, want %q", words, got, want)
		}
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
	base := write(t, dir, "kit.yml", "protected_branches: [main]\non_edit:\n  - globs: [a]\n    signal_fies: [a]\n")
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
	if cfg.LogMaxLines != 10000 || cfg.GateTimeout != 90 {
		t.Errorf("defaults: log_max_lines=%d gate_timeout=%d", cfg.LogMaxLines, cfg.GateTimeout)
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
