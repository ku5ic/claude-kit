// Package config loads kit.yml and the user's overlay into one typed Config.
package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Config mirrors kit.yml. Every key kit.yml may hold is a field here, so a
// strict decode reports an unknown or misspelled key instead of dropping it.
type Config struct {
	GlobalSkills      []string          `yaml:"global_skills"`
	SkillFileMap      []SkillFileRule   `yaml:"skill_file_map"`
	DependencySkills  []DependencyRule  `yaml:"dependency_skills"`
	SkillTriggers     map[string]string `yaml:"skill_triggers"`
	ProtectedBranches []string          `yaml:"protected_branches"`
	GlobalInstalls    []string          `yaml:"global_installs"`
	RootArtifactExts  []string          `yaml:"root_artifact_exts"`
	RCFiles           []string          `yaml:"rc_files"`
	SensitivePaths    []string          `yaml:"sensitive_paths"`
	LogMaxLines       int               `yaml:"log_max_lines"`
	DisabledRules     []string          `yaml:"disabled_rules"`

	GateDiscovery         GateDiscovery `yaml:"gate_discovery"`
	DenyFlags             []string      `yaml:"deny_flags"`
	GateEnv               []string      `yaml:"gate_env"`
	GateTimeout           int           `yaml:"gate_timeout"`
	DisabledTaskProviders []string      `yaml:"disabled_task_providers"`
	DisabledChecks        []string      `yaml:"disabled_checks"`

	ReplyLimits   ReplyLimits `yaml:"reply_limits"`
	SanitizeSkip  []string    `yaml:"sanitize_skip"`
	SkipDirs      []string    `yaml:"skip_dirs"`
	Tools         []string    `yaml:"tools"`
	LockfileGlobs []string    `yaml:"lockfile_globs"`
	Classifier    []string    `yaml:"classifier"`
	OnEdit        []EditRule  `yaml:"on_edit"`

	// Tag is "merged" when an overlay was merged in, else "base". Caches
	// derived from the config carry it in their file name, so deleting the
	// overlay can't leave its derived state in use.
	Tag string `yaml:"-"`
}

// CheckKinds are the kinds of check gap-fill sorts a check into.
var CheckKinds = []string{"lint", "typecheck", "test", "format-check", "deadcode", "security"}

// CheckDisabled is true when disabled_checks names a gate: by a kind it
// checks ("lint"), or by its label as run-checks prints it ("lint
// (package.json: lint:css) [web]"). A label without its " [dir]" part
// names the gate in every subproject.
func (c *Config) CheckDisabled(kind, label string) bool {
	for _, d := range c.DisabledChecks {
		if d != "" && (d == kind || d == label || d == rootLabel(label)) {
			return true
		}
	}
	return false
}

// rootLabel is label without the " [dir]" run-checks appends for a subproject.
func rootLabel(label string) string {
	if i := strings.LastIndex(label, " ["); i >= 0 && strings.HasSuffix(label, "]") {
		return label[:i]
	}
	return label
}

// unknownDisables names the disabled_checks entries that can name nothing:
// a single word that isn't a check kind, so a typo doesn't silently disable
// nothing. A label holds a space, and which gates have it depends on the
// project.
func (c *Config) unknownDisables() []error {
	var errs []error
	for _, d := range c.DisabledChecks {
		if !strings.Contains(d, " ") && !slices.Contains(CheckKinds, d) {
			errs = append(errs, fmt.Errorf("disabled_checks: %q is no check kind (%s) or gate label", d, strings.Join(CheckKinds, ", ")))
		}
	}
	return errs
}

type SkillFileRule struct {
	On     string   `yaml:"on"`
	Globs  []string `yaml:"globs"`
	Skills []string `yaml:"skills"`
}

// DependencyRule gives skills to a project declaring any of Deps.
type DependencyRule struct {
	Deps   []string `yaml:"deps"`
	Skills []string `yaml:"skills"`
}

// GateDiscovery is the CI steps and jobs the kit never runs (go/internal/ci).
type GateDiscovery struct {
	DenyCommands []string `yaml:"deny_commands"`
	DenyNames    []string `yaml:"deny_names"`
	DenyActions  []string `yaml:"deny_actions"`
}

// DeniedCommand is the deny_commands token words hold, one word anywhere
// or several in a row; "" when none.
func (g GateDiscovery) DeniedCommand(words []string) string {
	for _, token := range g.DenyCommands {
		t := strings.Fields(token)
		for i := 0; i+len(t) <= len(words); i++ {
			if slices.Equal(words[i:i+len(t)], t) {
				return token
			}
		}
	}
	return ""
}

// DeniedFlag is the first deny_flags flag words set: alone, or as
// flag=value with a value that doesn't turn it off (--watch=false doesn't);
// "" when none.
func (c *Config) DeniedFlag(words []string) string {
	for _, flag := range c.DenyFlags {
		if slices.ContainsFunc(words, func(w string) bool {
			value, ok := strings.CutPrefix(w, flag+"=")
			return w == flag || ok && !slices.Contains([]string{"false", "0", "no", "off"}, strings.ToLower(value))
		}) {
			return flag
		}
	}
	return ""
}

// EditRule is the user's policy for an edited file its globs match: Run
// formats it when no fixer of the project's claims it, the first command
// whose binary is on PATH; Note runs on it whatever claims it, its output
// shown, nothing changed.
type EditRule struct {
	Globs []string      `yaml:"globs"`
	Run   []EditCommand `yaml:"run"`
	Note  string        `yaml:"note"`
}

// EditCommand is one formatter in an EditRule's chain, {file} its path;
// Stdout replaces the file with what it prints.
type EditCommand struct {
	Cmd    string `yaml:"cmd"`
	Stdout bool   `yaml:"stdout"`
}

// ReplyLimits are the word ceilings rules/output.md section 0 names. A
// prompt holding a detail trigger lifts the chat ceiling, one holding an
// explain trigger raises it to Explain. Write is per /write kind, the whole
// reply. A prompt that starts with an UncappedCommands command has none.
// 0 is no ceiling.
type ReplyLimits struct {
	Chat             int            `yaml:"chat"`
	Explain          int            `yaml:"explain"`
	ExplainTriggers  []string       `yaml:"explain_triggers"`
	DetailTriggers   []string       `yaml:"detail_triggers"`
	UncappedCommands []string       `yaml:"uncapped_commands"`
	Write            map[string]int `yaml:"write"`
}

// negatives names each ceiling below 0, which would silently mean "off".
func (l ReplyLimits) negatives() []error {
	var errs []error
	check := func(name string, n int) {
		if n < 0 {
			errs = append(errs, fmt.Errorf("reply_limits: %s is %d; a ceiling is 0 (off) or more", name, n))
		}
	}
	check("chat", l.Chat)
	check("explain", l.Explain)
	for _, kind := range slices.Sorted(maps.Keys(l.Write)) {
		check("write."+kind, l.Write[kind])
	}
	return errs
}
