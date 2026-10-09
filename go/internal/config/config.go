// Package config loads kit.yml and the user's overlay into one typed Config.
package config

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Config mirrors kit.yml. Every key kit.yml may hold is a field here, so a
// strict decode reports an unknown or misspelled key instead of dropping it.
type Config struct {
	GlobalSkills      []string            `yaml:"global_skills"`
	SkillFileMap      []SkillFileRule     `yaml:"skill_file_map"`
	SkillTriggers     map[string]string   `yaml:"skill_triggers"`
	PackageManagers   []PackageManager    `yaml:"package_managers"`
	ExtraLockfiles    []string            `yaml:"extra_lockfiles"`
	ProtectedBranches []string            `yaml:"protected_branches"`
	DependencyAdds    map[string][]string `yaml:"dependency_adds"`
	RootArtifactExts  []string            `yaml:"root_artifact_exts"`
	RCFiles           []string            `yaml:"rc_files"`
	SensitivePaths    []string            `yaml:"sensitive_paths"`
	LogMaxLines       int                 `yaml:"log_max_lines"`
	DisabledRules     []string            `yaml:"disabled_rules"`
	TaskProviders     []TaskProvider      `yaml:"task_providers"`
	Checks            []Check             `yaml:"checks"`
	ToolchainChecks   []ToolchainCheck    `yaml:"toolchain_checks"`

	GateDiscovery           GateDiscovery `yaml:"gate_discovery"`
	DisabledTaskProviders   []string      `yaml:"disabled_task_providers"`
	DisabledChecks          []string      `yaml:"disabled_checks"`
	DisabledToolchainChecks []string      `yaml:"disabled_toolchain_checks"`

	SubprojectMaxDepth int                 `yaml:"subproject_max_depth"`
	Formatters         []Formatter         `yaml:"formatters"`
	DisabledFormatters []string            `yaml:"disabled_formatters"`
	FileChecks         []FileCheck         `yaml:"file_checks"`
	DisabledFileChecks []string            `yaml:"disabled_file_checks"`
	CheckTimeout       int                 `yaml:"check_timeout"`
	ReplyLimits        ReplyLimits         `yaml:"reply_limits"`
	SanitizeSkip       []string            `yaml:"sanitize_skip"`
	SkipDirs           []string            `yaml:"skip_dirs"`
	ToolResolution     ToolResolution      `yaml:"tool_resolution"`
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

// ToolchainEnabled is false for a toolchain check disabled_toolchain_checks
// names as "<stack>:<name>".
func (c *Config) ToolchainEnabled(tc ToolchainCheck) bool {
	return !slices.Contains(c.DisabledToolchainChecks, tc.Stack+":"+tc.Name)
}

// CheckDisabled is true when disabled_checks names a run-checks check: by
// its slot ("lint"), or by its label as run-checks prints it, with or
// without the "<stack>: " prefix ("lint (lint:css) [web]"). A label without
// its " [dir]" part names the check in every subproject.
func (c *Config) CheckDisabled(slot, label string) bool {
	short := stackPrefix.ReplaceAllString(label, "")
	for _, d := range c.DisabledChecks {
		if d == "" {
			continue
		}
		if d == slot || slices.Contains([]string{label, short, rootLabel(label), rootLabel(short)}, d) {
			return true
		}
	}
	return false
}

// stackPrefix is the "<stack>: " a run-checks label starts with; a ": "
// later in the label ("ci.yml: eslint") is part of its source.
var stackPrefix = regexp.MustCompile(`^[\w.-]+: `)

// rootLabel is label without the " [dir]" run-checks appends for a subproject.
func rootLabel(label string) string {
	if i := strings.LastIndex(label, " ["); i >= 0 && strings.HasSuffix(label, "]") {
		return label[:i]
	}
	return label
}

// unknownDisables names the disabled_checks, disabled_toolchain_checks, and
// disabled_task_providers entries that match nothing, so a typo doesn't
// silently disable nothing. A disabled_checks label is judged by its check
// name, the word its label starts with after any "<stack>: ".
func (c *Config) unknownDisables() []error {
	var errs []error
	known := func(name string) bool {
		return slices.ContainsFunc(c.Checks, func(ch Check) bool { return ch.Name == name }) ||
			slices.ContainsFunc(c.ToolchainChecks, func(tc ToolchainCheck) bool { return tc.Name == name })
	}
	for _, d := range c.DisabledChecks {
		if name, _, _ := strings.Cut(stackPrefix.ReplaceAllString(d, ""), " "); !known(name) {
			errs = append(errs, fmt.Errorf("disabled_checks: %q names no check", d))
		}
	}
	for _, d := range c.DisabledToolchainChecks {
		if !slices.ContainsFunc(c.ToolchainChecks, func(tc ToolchainCheck) bool { return tc.Stack+":"+tc.Name == d }) {
			errs = append(errs, fmt.Errorf("disabled_toolchain_checks: %q names no toolchain check (<stack>:<name>)", d))
		}
	}
	for _, d := range c.DisabledTaskProviders {
		if !slices.ContainsFunc(c.TaskProviders, func(tp TaskProvider) bool { return tp.Name == d }) {
			errs = append(errs, fmt.Errorf("disabled_task_providers: %q names no task provider", d))
		}
	}
	return errs
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

// pyDepFiles are the manifests a pydep rule reads (tools.PythonDeps), past
// requirements.txt's siblings.
var pyDepFiles = []string{"pyproject.toml", "requirements.txt", "Pipfile"}

// DetectFiles are the files whose change can change detection: every
// sentinel, every extra's file, grep, and pydep target, and every lockfile, deduped
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
			for _, r := range append([]Rule{e.Rule}, e.AnyOf...) {
				if r.PyDep != "" {
					for _, f := range pyDepFiles {
						add(f)
					}
				}
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
	// Per manager, on its first entry.
	Default       bool     `yaml:"default"`
	Aliases       []string `yaml:"aliases"`
	Dlx           string   `yaml:"dlx"`
	DirFlags      []string `yaml:"dir_flags"`
	GlobalInstall string   `yaml:"global_install"`
	// Its verbs where another manager's "install" means something else.
	AddVerb  string `yaml:"add_verb"`
	SyncVerb string `yaml:"sync_verb"`
}

// Manager is the first package_managers entry for a manager, or for a
// command that runs one (npx runs npm).
func (c *Config) Manager(command string) (PackageManager, bool) {
	for _, pm := range c.PackageManagers {
		if pm.Manager == command || slices.Contains(pm.Aliases, command) {
			return pm, true
		}
	}
	return PackageManager{}, false
}

// DefaultManager is an ecosystem's manager when no lockfile names one.
func (c *Config) DefaultManager(ecosystem string) string {
	for _, pm := range c.PackageManagers {
		if pm.Ecosystem == ecosystem && pm.Default {
			return pm.Manager
		}
	}
	return ""
}

type TaskProvider struct {
	Name      string            `yaml:"name"`
	Stack     string            `yaml:"stack"`
	Manifests []string          `yaml:"manifests"`
	Extractor string            `yaml:"extractor"`
	Arg       string            `yaml:"arg"`
	Run       string            `yaml:"run"`
	RunByPM   map[string]string `yaml:"run_by_pm"`
	Body      string            `yaml:"body"`
}

type Check struct {
	Name    string        `yaml:"name"`
	Tasks   []string      `yaml:"tasks"`
	Exclude []string      `yaml:"exclude"`
	Tools   []ToolPattern `yaml:"tools"`
	// FallbackTasks: names that count only when no Tasks glob matches, and
	// then only the first present; the rest are covered by it.
	FallbackTasks []string `yaml:"fallback_tasks"`
	// ExcludeDirs: globs matched against each path segment of a subproject;
	// a toolchain check standing in for this check skips a match.
	ExcludeDirs []string `yaml:"exclude_dirs"`
	// Scope "changed": the check runs whole-program, but only findings in
	// files changed since the git base fail it (dead code).
	Scope string `yaml:"scope"`
}

// ToolPattern is a command that counts as a check: bin by name, its first
// non-flag argument one of sub when sub is set, every require flag present,
// and no forbid flag (a flag counts as written alone or as flag=value).
// For a scoped check, Findings is a regex with file and line groups for one
// finding (a group name may repeat across alternatives); a flag in Unmapped changes the output so it no longer matches,
// and Advisory marks output that can't be mapped to files at all.
type ToolPattern struct {
	Bin      string   `yaml:"bin"`
	Sub      []string `yaml:"sub"`
	Require  []string `yaml:"require"`
	Forbid   []string `yaml:"forbid"`
	Findings string   `yaml:"findings"`
	Unmapped []string `yaml:"unmapped"`
	Advisory bool     `yaml:"advisory"`
}

// GateDiscovery is the shell grammar the gate classifier reads task bodies
// with (go/internal/classify).
type GateDiscovery struct {
	Wrappers       []string    `yaml:"wrappers"`
	ToolRunners    []string    `yaml:"tool_runners"`
	References     []Reference `yaml:"references"`
	ScriptRunners  []string    `yaml:"script_runners"`
	FanOutFlags    []string    `yaml:"fan_out_flags"`
	FanOutCommands []string    `yaml:"fan_out_commands"`
	DenyCommands   []string    `yaml:"deny_commands"`
	DenyNames      []string    `yaml:"deny_names"`
	DenyActions    []string    `yaml:"deny_actions"`
}

// Reference is a command prefix that runs another task of provider: the
// word after it, or Task when set. With Shorthand, a word that names no such
// task is a tool instead (pnpm eslint).
type Reference struct {
	Prefix    string `yaml:"prefix"`
	Provider  string `yaml:"provider"`
	Task      string `yaml:"task"`
	Shorthand bool   `yaml:"shorthand"`
}

type ToolchainCheck struct {
	Stack   string   `yaml:"stack"`
	Name    string   `yaml:"name"`
	Cmd     string   `yaml:"cmd"`
	Bin     []string `yaml:"bin"`
	WhenDir string   `yaml:"when_dir"`
	Slot    string   `yaml:"slot"`
}

type Formatter struct {
	Name           string   `yaml:"name"`
	Ext            []string `yaml:"ext"`
	SignalFiles    []string `yaml:"signal_files"`
	SignalTOML     string   `yaml:"signal_toml"`
	SignalPrettier bool     `yaml:"signal_prettier"`
	Bin            string   `yaml:"bin"`
	Cmd            string   `yaml:"cmd"`
	Fallback       bool     `yaml:"fallback"`
	Stdout         bool     `yaml:"stdout"`
}

// ToolResolution is where a tool's binary may come from beyond the
// project's own copy (go/internal/tools.Resolve).
type ToolResolution struct {
	PathFallback []string          `yaml:"path_fallback"`
	PinFiles     []string          `yaml:"pin_files"`
	PinAliases   map[string]string `yaml:"pin_aliases"`
	ManagerDirs  []string          `yaml:"manager_dirs"`
	BinPackages  map[string]string `yaml:"bin_packages"`
	Install      map[string]string `yaml:"install"`
	EnvLookups   []EnvLookup       `yaml:"env_lookups"`
}

// EnvLookup finds a binary inside a package manager's environment, when
// Marker is found walking up: VenvCmd prints the environment dir (the bin is
// <dir>/bin/<name>), or Probe exits 0 when the bin is there and Run runs it.
// {bin} is the binary's name.
type EnvLookup struct {
	Marker  string   `yaml:"marker"`
	VenvCmd []string `yaml:"venv_cmd"`
	Probe   []string `yaml:"probe"`
	Run     []string `yaml:"run"`
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

// Rule is one detection test of an extra: a package.json dep, a Python
// dependency (pydep), a file, or a grep over files.
type Rule struct {
	Dep   string   `yaml:"dep"`
	PyDep string   `yaml:"pydep"`
	File  string   `yaml:"file"`
	Grep  string   `yaml:"grep"`
	In    []string `yaml:"in"`
}

type Extra struct {
	Name   string `yaml:"name"`
	Rule   `yaml:",inline"`
	AnyOf  []Rule   `yaml:"any_of"`
	Skills []string `yaml:"skills"`
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
