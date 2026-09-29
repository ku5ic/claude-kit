// Package config loads kit.yml and the user's overlay into one typed Config.
package config

import (
	"os"
	"path/filepath"
)

// Config mirrors kit.yml. Every key kit.yml may hold is a field here, so a
// strict decode reports an unknown or misspelled key instead of dropping it.
type Config struct {
	GlobalSkills       []string            `yaml:"global_skills"`
	SkillFileMap       []SkillFileRule     `yaml:"skill_file_map"`
	SkillTriggers      map[string]string   `yaml:"skill_triggers"`
	PackageManagers    []PackageManager    `yaml:"package_managers"`
	ExtraLockfiles     []string            `yaml:"extra_lockfiles"`
	ProtectedBranches  []string            `yaml:"protected_branches"`
	RCFiles            []string            `yaml:"rc_files"`
	SensitivePaths     []string            `yaml:"sensitive_paths"`
	LogMaxLines        int                 `yaml:"log_max_lines"`
	DisabledRules      []string            `yaml:"disabled_rules"`
	TaskProviders      []TaskProvider      `yaml:"task_providers"`
	Checks             []Check             `yaml:"checks"`
	ToolchainChecks    []ToolchainCheck    `yaml:"toolchain_checks"`
	SubprojectMaxDepth int                 `yaml:"subproject_max_depth"`
	Formatters         []Formatter         `yaml:"formatters"`
	DisabledFormatters []string            `yaml:"disabled_formatters"`
	FileChecks         []FileCheck         `yaml:"file_checks"`
	DisabledFileChecks []string            `yaml:"disabled_file_checks"`
	CheckTimeout       int                 `yaml:"check_timeout"`
	Tools              []string            `yaml:"tools"`
	Orchestrators      []Orchestrator      `yaml:"orchestrators"`
	Versions           map[string][]string `yaml:"versions"`
	VersionSources     map[string][]Source `yaml:"version_sources"`
	Stacks             map[string]Stack    `yaml:"stacks"`

	// Document order of the map-keyed sections, which a Go map loses:
	// detection reports stacks, and versions, in the order kit.yml (then
	// the overlay) lists them.
	StackOrder   []string `yaml:"-"`
	VersionOrder []string `yaml:"-"`
	// Tag is "merged" when an overlay was merged in, else "base". Caches
	// derived from the config carry it in their file name, so deleting the
	// overlay can't leave its derived state in use.
	Tag string `yaml:"-"`
}

// AnchorSentinels are the sentinels marked anchor: true, in stack order:
// the files that make a directory a project root or a subproject.
func (c *Config) AnchorSentinels() []string {
	var out []string
	for _, name := range c.StackOrder {
		for _, s := range c.Stacks[name].Sentinels {
			if s.Anchor {
				out = append(out, s.Name)
			}
		}
	}
	return out
}

// DetectFiles are the files whose change can change detection: every
// sentinel, every extra's file and grep target, and every lockfile, deduped
// in first-seen order.
func (c *Config) DetectFiles() []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	for _, name := range c.StackOrder {
		for _, s := range c.Stacks[name].Sentinels {
			add(s.Name)
		}
	}
	for _, name := range c.StackOrder {
		for _, e := range c.Stacks[name].Extras {
			add(e.File)
			for _, f := range e.In {
				add(f)
			}
			for _, r := range e.AnyOf {
				add(r.File)
				for _, f := range r.In {
					add(f)
				}
			}
		}
	}
	for _, pm := range c.PackageManagers {
		add(pm.Lockfile)
	}
	return out
}

// HasStack is true when dir holds a sentinel of stack.
func (c *Config) HasStack(dir, stack string) bool {
	for _, s := range c.Stacks[stack].Sentinels {
		if info, err := os.Stat(filepath.Join(dir, s.Name)); err == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
}

type SkillFileRule struct {
	On     string   `yaml:"on"`
	Globs  []string `yaml:"globs"`
	Skills []string `yaml:"skills"`
}

type PackageManager struct {
	Lockfile   string `yaml:"lockfile"`
	Manager    string `yaml:"manager"`
	Ecosystem  string `yaml:"ecosystem"`
	HandEdited bool   `yaml:"hand_edited"`
}

type TaskProvider struct {
	Name      string            `yaml:"name"`
	Stack     string            `yaml:"stack"`
	Manifests []string          `yaml:"manifests"`
	Extractor string            `yaml:"extractor"`
	Arg       string            `yaml:"arg"`
	Run       string            `yaml:"run"`
	RunByPM   map[string]string `yaml:"run_by_pm"`
}

type Check struct {
	Name    string   `yaml:"name"`
	Tasks   []string `yaml:"tasks"`
	Exclude []string `yaml:"exclude"`
}

type ToolchainCheck struct {
	Stack   string   `yaml:"stack"`
	Name    string   `yaml:"name"`
	Cmd     string   `yaml:"cmd"`
	Bin     []string `yaml:"bin"`
	WhenDir string   `yaml:"when_dir"`
}

type Formatter struct {
	Name           string   `yaml:"name"`
	Ext            []string `yaml:"ext"`
	SignalFiles    []string `yaml:"signal_files"`
	SignalTOML     string   `yaml:"signal_toml"`
	SignalPrettier bool     `yaml:"signal_prettier"`
	Bin            string   `yaml:"bin"`
	Cmd            string   `yaml:"cmd"`
}

// FileCheck is a user-defined file-scoped check (kit.yml file_checks); the
// built-in ones are go/internal/tools adapters.
type FileCheck struct {
	Name        string   `yaml:"name"`
	Ext         []string `yaml:"ext"`
	SignalFiles []string `yaml:"signal_files"`
	SignalTOML  string   `yaml:"signal_toml"`
	ExcludeTOML string   `yaml:"exclude_toml"`
	LocalOnly   bool     `yaml:"local_only"`
	Bin         string   `yaml:"bin"`
	Cmd         string   `yaml:"cmd"`
}

type Orchestrator struct {
	Name      string   `yaml:"name"`
	Signal    string   `yaml:"signal"`
	TaskPaths []string `yaml:"task_paths"`
	Run       string   `yaml:"run"`
}

type Source struct {
	File      string `yaml:"file"`
	Extractor string `yaml:"extractor"`
	Arg       string `yaml:"arg"`
	Label     string `yaml:"label"`
	Up        bool   `yaml:"up"`
}

type Stack struct {
	Sentinels []Sentinel `yaml:"sentinels"`
	Skills    []string   `yaml:"skills"`
	Extras    []Extra    `yaml:"extras"`
}

type Sentinel struct {
	Name   string `yaml:"name"`
	Anchor bool   `yaml:"anchor"`
}

// Rule is one detection test of an extra: a dep, a file, or a grep over files.
type Rule struct {
	Dep  string   `yaml:"dep"`
	File string   `yaml:"file"`
	Grep string   `yaml:"grep"`
	In   []string `yaml:"in"`
}

type Extra struct {
	Name   string `yaml:"name"`
	Rule   `yaml:",inline"`
	AnyOf  []Rule   `yaml:"any_of"`
	Rename string   `yaml:"rename"`
	Skills []string `yaml:"skills"`
}
