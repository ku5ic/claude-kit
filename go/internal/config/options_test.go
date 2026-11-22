package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

const optionsManifest = `{"name": "kit", "userConfig": {
  "preset": {"default": "default"},
  "step_pause": {"default": "default"},
  "receipt": {"default": "default"},
  "subagent_rows": {"default": true}
}}`

// optionPaths is presetPaths plus the plugin manifest and a settings.json
// holding settings, whose pluginConfigs are the /config options.
func optionPaths(t *testing.T, base, light, settings string) Paths {
	t.Helper()
	p := presetPaths(t, base, light)
	write(t, filepath.Join(p.Root, "presets"), "standard.yml", "# kit.yml as it is\n")
	if err := os.Mkdir(filepath.Join(p.Root, ".claude-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	p.Manifest = write(t, filepath.Join(p.Root, ".claude-plugin"), "plugin.json", optionsManifest)
	p.Settings = write(t, p.ClaudeDir, "settings.json", settings)
	return p
}

func loadClean(t *testing.T, p Paths) *Config {
	t.Helper()
	cfg, warnings, err := Load(p)
	if err != nil || len(warnings) > 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	return cfg
}

func TestAConfigOptionBeatsTheOverlayAndDefaultSetsNothing(t *testing.T) {
	p := optionPaths(t, "step_pause: step\nreceipt: all\nglobal_installs: []\n", "",
		`{"pluginConfigs": {"kit@market": {"options": {
		  "step_pause": "plan", "receipt": "default", "subagent_rows": false,
		  "global_installs": ["undeclared"]}}}}`)
	write(t, p.Root, "claude-kit.local.yml", "step_pause: off\nreceipt: failures\n")
	cfg := loadClean(t, p)
	if cfg.StepPause != "plan" || cfg.Receipt != "failures" || len(cfg.GlobalInstalls) != 0 {
		t.Errorf("step_pause=%q receipt=%q global_installs=%v, want /config's plan, the overlay's failures, and no undeclared option", cfg.StepPause, cfg.Receipt, cfg.GlobalInstalls)
	}
}

func TestTheOptionsPresetPicksThePresetFile(t *testing.T) {
	for _, tc := range []struct {
		name, overlay, option string
		want                  int
	}{
		{"with no overlay", "", "light", 6},
		{"over the overlay's", "preset: light\n", "standard", 5},
		{"default leaves the overlay's", "preset: light\n", "default", 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := optionPaths(t, "log_max_lines: 5\n", "log_max_lines: 6\n",
				`{"pluginConfigs": {"kit@market": {"options": {"preset": "`+tc.option+`"}}}}`)
			if tc.overlay != "" {
				write(t, p.Root, "claude-kit.local.yml", tc.overlay)
			}
			if cfg := loadClean(t, p); cfg.LogMaxLines != tc.want {
				t.Errorf("log_max_lines = %d, want %d", cfg.LogMaxLines, tc.want)
			}
		})
	}
}

func TestOnlyTheKitsOneEntryApplies(t *testing.T) {
	for _, tc := range []struct {
		name, settings, want, warning string
	}{
		{"another plugin's entry is ignored", `{"pluginConfigs": {"other@market": {"options": {"step_pause": "off"}}}}`, "step", ""},
		{"two kit entries warn and apply neither", `{"pluginConfigs": {"kit@a": {"options": {"step_pause": "off"}}, "kit@b": {"options": {"step_pause": "plan"}}}}`, "step", "kit@a, kit@b"},
		{"broken JSON warns", `{"pluginConfigs": `, "step", "ignored"},
		{"no pluginConfigs is quiet", `{"theme": "dark"}`, "step", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := optionPaths(t, "step_pause: step\n", "", tc.settings)
			cfg, warnings, err := Load(p)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.StepPause != tc.want {
				t.Errorf("step_pause = %q, want %q", cfg.StepPause, tc.want)
			}
			switch {
			case tc.warning == "" && len(warnings) > 0:
				t.Errorf("warnings = %v, want none", warnings)
			case tc.warning != "" && (len(warnings) != 1 || warnings[0].File != p.Settings || !strings.Contains(warnings[0].Err.Error(), tc.warning)):
				t.Errorf("warnings = %v, want one on settings.json naming %q", warnings, tc.warning)
			}
		})
	}
}

func TestLoadCacheFollowsSettings(t *testing.T) {
	p := optionPaths(t, "step_pause: step\n", "", `{}`)
	if got := loadClean(t, p).StepPause; got != "step" {
		t.Fatalf("first load step_pause = %q", got)
	}
	write(t, p.ClaudeDir, "settings.json", `{"pluginConfigs": {"kit@market": {"options": {"step_pause": "off"}}}}`)
	if got := loadClean(t, p).StepPause; got != "off" {
		t.Errorf("after a /config change step_pause = %q, want off", got)
	}
}

// The overlay is found by its path, so a bad pattern it adds still names it
// with the options layer merged after it.
func TestAnOverlayWarningStillNamesTheOverlayUnderOptions(t *testing.T) {
	p := optionPaths(t, "global_installs: ['npm (i|install) -g']\nstep_pause: step\n", "",
		`{"pluginConfigs": {"kit@market": {"options": {"step_pause": "plan"}}}}`)
	overlay := write(t, p.Root, "claude-kit.local.yml", "global_installs: ['pip install (--user']\n")
	_, warnings, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || warnings[0].File != overlay {
		t.Errorf("warnings = %v, want one naming the overlay", warnings)
	}
}

// With only /config set, a bad value names settings.json, not an overlay
// that doesn't exist.
func TestAnOptionsOnlyWarningNamesSettings(t *testing.T) {
	p := optionPaths(t, "step_pause: step\n", "",
		`{"pluginConfigs": {"kit@market": {"options": {"step_pause": "sometimes"}}}}`)
	_, warnings, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || warnings[0].File != p.Settings {
		t.Errorf("warnings = %v, want one naming settings.json", warnings)
	}
}

func TestMergedListsSettingsAmongTheSources(t *testing.T) {
	p := optionPaths(t, "step_pause: step\n", "",
		`{"pluginConfigs": {"kit@market": {"options": {"step_pause": "plan"}}}}`)
	write(t, p.Root, "claude-kit.local.yml", "step_pause: off\n")
	merged, err := Merged(p)
	if err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Marshal(merged)
	if err != nil {
		t.Fatal(err)
	}
	if want := "step_pause: plan # kit.yml, claude-kit.local.yml, settings.json"; !strings.Contains(string(out), want) {
		t.Errorf("missing %q in:\n%s", want, out)
	}
}
