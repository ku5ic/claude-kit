// Package config loads kit.yml and the user's overlay into one typed Config.
package config

import (
	"fmt"
	"maps"
	"regexp"
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
	Receipt               string        `yaml:"receipt"`
	UserPins              []string      `yaml:"user_pins"`
	DisabledTaskProviders []string      `yaml:"disabled_task_providers"`
	DisabledChecks        []string      `yaml:"disabled_checks"`

	ReplyLimits    ReplyLimits    `yaml:"reply_limits"`
	SanitizeSkip   []string       `yaml:"sanitize_skip"`
	SkipDirs       []string       `yaml:"skip_dirs"`
	Tools          []string       `yaml:"tools"`
	LockfileGlobs  []string       `yaml:"lockfile_globs"`
	Classifier     []string       `yaml:"classifier"`
	OnEdit         []EditRule     `yaml:"on_edit"`
	Slop           Slop           `yaml:"slop"`
	Connectors     Connectors     `yaml:"connectors"`
	Claims         Claims         `yaml:"claims"`
	WeakenedChecks WeakenedChecks `yaml:"weakened_checks"`
	PackageLookups PackageLookups `yaml:"package_lookups"`
	ChatBundle     ChatBundle     `yaml:"chat_bundle"`

	// Tag is "merged" when an overlay was merged in, else "base". Caches
	// derived from the config carry it in their file name, so deleting the
	// overlay can't leave its derived state in use.
	Tag string `yaml:"-"`

	// OverlayInstalls counts the GlobalInstalls entries, at its end, that
	// the overlay added. Exported so the on-disk cache keeps it.
	OverlayInstalls int `yaml:"-"`

	// installs is GlobalInstalls compiled, once, by load.
	installs *installPatterns
}

type installPatterns struct {
	patterns []*regexp.Regexp
	bad      []Warning
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

// Rules are the slugs disabled_rules can name, besides slop- and a slop
// rule's id. TestRulesMatchTheCode holds it to the code's blocks; a slug
// from a table or a variable is added by hand.
var Rules = []string{
	"ai-commit-sig", "aws-ec2-terminate", "aws-s3-force-rb", "aws-s3-recursive-rm",
	"checks-failed", "chmod-777", "chmod-broad-x", "claims", "commit-msg", "commit-wall-of-text",
	"device-write", "disk-tool", "docker-prune-all-force", "download-to-repo",
	"find-delete", "find-exec-rm", "fork-bomb",
	"gcloud-delete", "git-config-global", "git-dir-edit", "git-force-push", "git-no-verify",
	"git-push-protected", "git-reset-hard", "git-tree-discard", "global-installs-invalid",
	"interpreter-c-wrap", "keychain-delete", "kubectl-delete", "lockfile-edit",
	"pipe-to-shell", "pkg-global-install", "pkg-missing", "pkg-undeclared", "plan-critic", "plan-done",
	"pm-mismatch", "psql-destructive", "rc-edit", "rc-inplace-edit", "rc-redirect", "redis-destructive",
	"review-checks", "rm-recursive", "rules-gate", "run-checks-by-hook", "sensitive-read", "sensitive-write",
	"skills-gate", "slop", "slop-vocabulary", "staged-secret", "terraform-auto-approve", "terraform-destroy",
	"ui-undefined-token", "weaken", "xargs-rm",
}

// unknownDisables names the disabled_checks and disabled_rules entries that
// can name nothing, so a typo doesn't silently disable nothing. A
// disabled_checks entry with a space is a gate label, and which gates have
// it depends on the project.
func (c *Config) unknownDisables() []error {
	var errs []error
	for _, d := range c.DisabledChecks {
		if !strings.Contains(d, " ") && !slices.Contains(CheckKinds, d) {
			errs = append(errs, fmt.Errorf("disabled_checks: %q is no check kind (%s) or gate label", d, strings.Join(CheckKinds, ", ")))
		}
	}
	for _, d := range c.DisabledRules {
		id, slop := strings.CutPrefix(d, "slop-")
		known := slices.Contains(Rules, d) || slop && slices.ContainsFunc(c.Slop.Rules, func(r SlopRule) bool { return r.ID == id })
		if !known {
			errs = append(errs, fmt.Errorf("disabled_rules: %q is no rule; kit explain prints a block's rule", d))
		}
	}
	return errs
}

// dropEmptyEditCmds removes the on_edit run entries with no cmd and names
// each, so a rule left with none never replaces an earlier rule's chain.
func (c *Config) dropEmptyEditCmds() []error {
	var errs []error
	for r := range c.OnEdit {
		rule := &c.OnEdit[r]
		var kept []EditCommand
		for i, cmd := range rule.Run {
			if strings.TrimSpace(cmd.Cmd) == "" {
				errs = append(errs, fmt.Errorf("on_edit: the rule for %q has a run entry with no cmd (entry %d of %d); it's dropped", rule.Globs, i+1, len(rule.Run)))
				continue
			}
			kept = append(kept, cmd)
		}
		rule.Run = kept
	}
	return errs
}

// InstallPatterns is global_installs compiled, and a warning naming the
// file of each pattern that doesn't compile: guard-bash blocks every
// command while one is there. A Config load built is compiled once; any
// other, per call.
func (c *Config) InstallPatterns(p Paths) ([]*regexp.Regexp, []Warning) {
	if c.installs == nil {
		return c.compileInstalls(p)
	}
	return c.installs.patterns, c.installs.bad
}

func (c *Config) compileInstalls(p Paths) (patterns []*regexp.Regexp, bad []Warning) {
	fromOverlay := len(c.GlobalInstalls) - c.OverlayInstalls
	for i, expr := range c.GlobalInstalls {
		re, err := regexp.Compile(expr)
		if err != nil {
			file := p.Base
			if i >= fromOverlay {
				file = p.Overlay
			}
			bad = append(bad, Warning{file, fmt.Errorf("global_installs: %q doesn't compile, so guard-bash blocks every command: %w", expr, err)})
			continue
		}
		patterns = append(patterns, re)
	}
	return patterns, bad
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

// GateDiscovery is the CI steps and jobs the kit never runs (go/internal/sources/ci.go).
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

// ChatBundle is what go/internal/chatbundle ships to claude.ai chat.
type ChatBundle struct {
	Description string   `yaml:"description"`
	Skills      []string `yaml:"skills"`
	Rules       []string `yaml:"rules"`
}

// SlopSurfaces are where text is scanned for AI tells: a written file, a
// commit message, PR text, a subagent's report, a chat reply, and ui, the
// lines of a UI file as written, which only rules naming it scan.
var SlopSurfaces = []string{"file", "commit", "pr", "report", "chat", "ui"}

// Connectors are the verbs go/internal/connector reads an MCP tool's name
// by, and CodeFields, the input fields that hold code, not prose.
type Connectors struct {
	WriteVerbs []string `yaml:"write_verbs"`
	ReadVerbs  []string `yaml:"read_verbs"`
	CodeFields []string `yaml:"code_fields"`
}

// Claims is go/internal/claims's verifier: the model and spend cap passed
// to its claude -p call, its time limit in seconds, the tools it may call
// besides its own read-only set, and Command, which replaces the call.
type Claims struct {
	Command   []string `yaml:"command"`
	Model     string   `yaml:"model"`
	Timeout   int      `yaml:"timeout"`
	BudgetUSD float64  `yaml:"budget_usd"`
	Allow     []string `yaml:"allow"`
}

// problems names a limit below 0, which would silently mean "off": 0, or
// no claims block at all, turns the verifier off.
func (c Claims) problems() []error {
	var errs []error
	if c.Timeout < 0 {
		errs = append(errs, fmt.Errorf("claims: timeout is %d; 0 turns the verifier off", c.Timeout))
	}
	if c.BudgetUSD < 0 {
		errs = append(errs, fmt.Errorf("claims: budget_usd is %g; 0 turns the verifier off", c.BudgetUSD))
	}
	return errs
}

// Off is true when the verifier has no time or no money to run with.
func (c Claims) Off() bool { return c.Timeout <= 0 || c.BudgetUSD <= 0 }

// WeakenedChecks are the paths and patterns go/internal/weaken reads an
// edit by: the files it skips, the files that are tests, and the lines
// that skip a test, silence a check, or assert.
type WeakenedChecks struct {
	Skip         []string `yaml:"skip"`
	TestFiles    []string `yaml:"test_files"`
	Skips        []string `yaml:"skips"`
	Suppressions []string `yaml:"suppressions"`
	Assertions   []string `yaml:"assertions"`
}

// problems names each pattern that doesn't compile.
func (w WeakenedChecks) problems() []error {
	var errs []error
	for _, l := range []struct {
		name     string
		patterns []string
	}{{"skips", w.Skips}, {"suppressions", w.Suppressions}, {"assertions", w.Assertions}} {
		for _, p := range l.patterns {
			if _, err := regexp.Compile(p); err != nil {
				errs = append(errs, fmt.Errorf("weakened_checks.%s: %q doesn't compile", l.name, p))
			}
		}
	}
	return errs
}

// PackageLookups are the registry lookups go/internal/pkgcheck runs on a
// package an add verb names: Timeout seconds for all of one command's
// lookups (0 turns them off), and each ecosystem's lookup.
type PackageLookups struct {
	Timeout    int                      `yaml:"timeout"`
	Ecosystems map[string]PackageLookup `yaml:"ecosystems"`
}

// PackageLookup is one ecosystem's: the managers it covers, the pattern a
// package name matches, the command ({pkg} is the name), what its output
// holds when no such package exists, and whether a missing path is tried
// again at each parent, as a Go package path inside a module.
type PackageLookup struct {
	Managers []string `yaml:"managers"`
	Name     string   `yaml:"name"`
	Cmd      []string `yaml:"cmd"`
	Missing  string   `yaml:"missing"`
	Parents  bool     `yaml:"parents"`
	// Private maps a registry config file to the line pattern ({scope} is
	// the package's scope, without @) that sends a package somewhere the
	// lookup, run outside the project, can't see.
	Private map[string]string `yaml:"private"`
	// PrivateCmd prints comma-separated globs of private package paths.
	PrivateCmd []string `yaml:"private_cmd"`
	// ValueFlags take the next word as their value, never a package.
	ValueFlags []string `yaml:"value_flags"`
	// RegistryFlags send the command's packages to another registry.
	RegistryFlags []string `yaml:"registry_flags"`
}

// Flags are the value-taking and registry flags of every ecosystem
// covering manager.
func (p PackageLookups) Flags(manager string) (value, registry []string) {
	for _, e := range p.Ecosystems {
		if slices.Contains(e.Managers, manager) {
			value = append(value, e.ValueFlags...)
			registry = append(registry, e.RegistryFlags...)
		}
	}
	return value, registry
}

// problems names each lookup that can't work: a pattern that doesn't
// compile, or a command that never names the package.
func (p PackageLookups) problems() []error {
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(p.Ecosystems)) {
		l := p.Ecosystems[name]
		for _, f := range [][2]string{{"name", l.Name}, {"missing", l.Missing}} {
			if _, err := regexp.Compile(f[1]); err != nil || f[1] == "" {
				errs = append(errs, fmt.Errorf("package_lookups.%s: %s %q doesn't compile", name, f[0], f[1]))
			}
		}
		for _, file := range slices.Sorted(maps.Keys(l.Private)) {
			if _, err := regexp.Compile(strings.ReplaceAll(l.Private[file], "{scope}", "x")); err != nil {
				errs = append(errs, fmt.Errorf("package_lookups.%s: private %s pattern doesn't compile", name, file))
			}
		}
		if !slices.ContainsFunc(l.Cmd, func(arg string) bool { return strings.Contains(arg, "{pkg}") }) {
			errs = append(errs, fmt.Errorf("package_lookups.%s: cmd never names {pkg}", name))
		}
	}
	return errs
}

// Slop is the AI-tell lexicon go/internal/slop scans with. Hits in a
// BlockTiers tier block; the rest are logged. Tier 3 rules and Density.Words
// only count toward the density score, never block on their own.
type Slop struct {
	BlockTiers []int       `yaml:"block_tiers"`
	Skip       []string    `yaml:"skip"`
	Density    SlopDensity `yaml:"density"`
	Rules      []SlopRule  `yaml:"rules"`
}

// SlopDensity flags a text once it holds at least Min tier 3 hits and at
// least Threshold of them per PerWords prose words.
type SlopDensity struct {
	PerWords  int      `yaml:"per_words"`
	Threshold int      `yaml:"threshold"`
	Min       int      `yaml:"min"`
	Words     []string `yaml:"words"`
}

// SlopRule is one tell: a regexp, the tier it belongs to, the surfaces it
// applies to (all when empty), and the rewrite direction a hit reports.
type SlopRule struct {
	ID    string   `yaml:"id"`
	Tier  int      `yaml:"tier"`
	On    []string `yaml:"on"`
	Match string   `yaml:"match"`
	Fix   string   `yaml:"fix"`
}

// problems names each rule that can't work as written: no id or a repeated
// one, a tier outside 1-3, an unknown surface, a regexp that doesn't
// compile, or a tier 1-2 rule with no fix to report.
func (s Slop) problems() []error {
	var errs []error
	seen := map[string]bool{}
	for i, r := range s.Rules {
		name := r.ID
		if name == "" {
			name = fmt.Sprintf("#%d", i+1)
			errs = append(errs, fmt.Errorf("slop.rules %s: no id", name))
		} else if seen[name] {
			errs = append(errs, fmt.Errorf("slop.rules %s: id repeated", name))
		}
		seen[name] = true
		if r.Tier < 1 || r.Tier > 3 {
			errs = append(errs, fmt.Errorf("slop.rules %s: tier %d; a tier is 1, 2, or 3", name, r.Tier))
		}
		for _, on := range r.On {
			if !slices.Contains(SlopSurfaces, on) {
				errs = append(errs, fmt.Errorf("slop.rules %s: %q is no surface (%s)", name, on, strings.Join(SlopSurfaces, ", ")))
			}
		}
		if _, err := regexp.Compile(r.Match); err != nil || r.Match == "" {
			errs = append(errs, fmt.Errorf("slop.rules %s: match doesn't compile: %q", name, r.Match))
		}
		if r.Tier < 3 && r.Fix == "" {
			errs = append(errs, fmt.Errorf("slop.rules %s: a tier %d rule needs a fix", name, r.Tier))
		}
	}
	for _, t := range s.BlockTiers {
		if t < 1 || t > 2 {
			errs = append(errs, fmt.Errorf("slop.block_tiers: %d; only tiers 1 and 2 block", t))
		}
	}
	return errs
}

// ReplyLimits are the word ceilings skills/kit-rules/reference/output.md section 0 names. A
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
