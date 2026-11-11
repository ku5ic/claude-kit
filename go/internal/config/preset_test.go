package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// presetPaths is a kit root in a temp dir with kit.yml, one preset file
// called light, and the overlay path.
func presetPaths(t *testing.T, base, light string) Paths {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "presets"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "presets"), "light.yml", light)
	return Paths{Root: dir, Home: dir, Base: write(t, dir, "kit.yml", base), Overlay: filepath.Join(dir, "claude-kit.local.yml")}
}

func TestPresetMergesBetweenKitYMLAndTheOverlay(t *testing.T) {
	p := presetPaths(t, "log_max_lines: 5\ngate_timeout: 30\ndisabled_rules: [plan-critic]\nstep_pause: step\n",
		"log_max_lines: 6\ngate_timeout: 40\ndisabled_rules: [claims]\nstep_pause: plan\n")
	write(t, p.Root, "claude-kit.local.yml", "preset: light\nlog_max_lines: 7\n")
	cfg, warnings, err := Load(p)
	if err != nil || len(warnings) > 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	if cfg.LogMaxLines != 7 || cfg.GateTimeout != 40 || cfg.StepPause != "plan" {
		t.Errorf("log_max_lines=%d gate_timeout=%d step_pause=%q, want the overlay's 7, the preset's 40 and plan", cfg.LogMaxLines, cfg.GateTimeout, cfg.StepPause)
	}
	if !slices.Equal(cfg.DisabledRules, []string{"plan-critic", "claims"}) || cfg.Tag != "merged" {
		t.Errorf("disabled_rules %v, tag %q", cfg.DisabledRules, cfg.Tag)
	}
}

func TestNoPresetLeavesThePresetFileOut(t *testing.T) {
	p := presetPaths(t, "log_max_lines: 5\n", "log_max_lines: 6\n")
	write(t, p.Root, "claude-kit.local.yml", "gate_timeout: 3\n")
	cfg, _, err := Load(p)
	if err != nil || cfg.LogMaxLines != 5 {
		t.Errorf("err=%v log_max_lines=%d, want kit.yml's 5", err, cfg.LogMaxLines)
	}
}

func TestAnUnknownPresetWarnsAndTheOverlayStillApplies(t *testing.T) {
	for _, name := range []string{"heavy", "../kit"} {
		p := presetPaths(t, "log_max_lines: 5\n", "")
		write(t, p.Root, "claude-kit.local.yml", "preset: "+name+"\nlog_max_lines: 7\n")
		cfg, warnings, err := Load(p)
		if err != nil || cfg.LogMaxLines != 7 {
			t.Fatalf("%s: err=%v log_max_lines=%d", name, err, cfg.LogMaxLines)
		}
		if len(warnings) != 1 || warnings[0].File != p.Overlay || !strings.Contains(warnings[0].Err.Error(), "no preset") {
			t.Errorf("%s: warnings = %v, want one on the overlay naming no preset", name, warnings)
		}
	}
}

func TestAPresetFileIsCheckedForUnknownKeys(t *testing.T) {
	p := presetPaths(t, "log_max_lines: 5\n", "log_max_line: 6\n")
	write(t, p.Root, "claude-kit.local.yml", "preset: light\n")
	_, warnings, err := Load(p)
	if err != nil || len(warnings) != 1 || !strings.HasSuffix(warnings[0].File, filepath.Join("presets", "light.yml")) {
		t.Errorf("err=%v warnings=%v, want one naming presets/light.yml", err, warnings)
	}
}

func TestABadStepPauseWarns(t *testing.T) {
	p := presetPaths(t, "step_pause: sometimes\n", "")
	cfg, warnings, err := Load(p)
	if err != nil || len(warnings) != 1 || !strings.Contains(warnings[0].Err.Error(), "step_pause") {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	if cfg.Pause() != "step" {
		t.Errorf("Pause() = %q, want the step default", cfg.Pause())
	}
}

func TestLoadCacheFollowsThePresetFile(t *testing.T) {
	p := presetPaths(t, "log_max_lines: 5\n", "log_max_lines: 6\n")
	write(t, p.Root, "claude-kit.local.yml", "preset: light\n")
	for _, want := range []int{6, 6} { // fresh, then cached
		if cfg, _, _ := Load(p); cfg.LogMaxLines != want {
			t.Errorf("log_max_lines = %d, want %d", cfg.LogMaxLines, want)
		}
	}
	light := filepath.Join(p.Root, "presets", "light.yml")
	write(t, filepath.Dir(light), "light.yml", "log_max_lines: 66\n")
	if cfg, _, _ := Load(p); cfg.LogMaxLines != 66 {
		t.Errorf("after the preset changes, log_max_lines = %d, want 66", cfg.LogMaxLines)
	}
}

func TestMergedNamesWhereEachKeyCameFrom(t *testing.T) {
	p := presetPaths(t, "log_max_lines: 5\ngate_timeout: 30\ntools: [rg]\n", "gate_timeout: 40\nstep_pause: plan\n")
	write(t, p.Root, "claude-kit.local.yml", "preset: light\nlog_max_lines: 7\ntools: [fd]\n")
	merged, err := Merged(p)
	if err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Marshal(merged)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"log_max_lines: 7 # kit.yml, claude-kit.local.yml",
		"gate_timeout: 40 # kit.yml, presets/light.yml",
		"step_pause: plan # presets/light.yml",
		"preset: light # claude-kit.local.yml",
		"tools: [rg, fd] # kit.yml, claude-kit.local.yml",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// The shipped presets hold only keys kit.yml knows, and the strict one turns
// both opt-in guards on.
func TestShippedPresetsLoadCleanly(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(root, "presets", "*.yml"))
	if len(files) != 3 {
		t.Fatalf("presets = %v, want light, standard, strict", files)
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".yml")
		dir := t.TempDir()
		p := Paths{Root: root, Home: dir, Base: filepath.Join(root, "kit.yml"), Overlay: write(t, dir, "claude-kit.local.yml", "preset: "+name+"\n")}
		cfg, warnings, err := Load(p)
		if err != nil || len(warnings) > 0 {
			t.Errorf("%s: err=%v warnings=%v", name, err, warnings)
			continue
		}
		if name == "strict" && (!cfg.GuardSkills || !cfg.SanitizeTypography) {
			t.Errorf("strict: guard_skills=%v sanitize_typography=%v", cfg.GuardSkills, cfg.SanitizeTypography)
		}
	}
}
