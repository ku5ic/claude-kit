package config

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

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
	PathFields []string `yaml:"path_fields"`
	// PathServers are the MCP servers whose tools touch local files, by
	// name, alone or as a plugin server's last part.
	PathServers []string `yaml:"path_servers"`
}

// Attribution is what go/internal/attribution finds an AI tool's
// attribution by: Patterns, where {tools} and {tool} stand for any of Tools.
type Attribution struct {
	Tools    []string `yaml:"tools"`
	Patterns []string `yaml:"patterns"`
}

// Expanded is Patterns with {tools} and {tool} replaced, each
// case-insensitive. A {tools} match ends at anything but a letter, digit,
// or hyphen, so claude-kit names no tool; a {tool} match may run on into a
// hyphenated word (Claude-Session). A space in a tool matches any run of
// spaces.
func (a Attribution) Expanded() []string {
	names := make([]string, len(a.Tools))
	for i, t := range a.Tools {
		names[i] = strings.Join(strings.Fields(regexp.QuoteMeta(t)), `\s+`)
	}
	tool := `\b(?:` + strings.Join(names, "|") + `)`
	tools := tool + `(?:[^\w-]|$)`
	out := make([]string, len(a.Patterns))
	for i, p := range a.Patterns {
		out[i] = "(?i)" + strings.NewReplacer("{tools}", tools, "{tool}", tool).Replace(p)
	}
	return out
}

// problems names each pattern that doesn't compile.
func (a Attribution) problems() []error {
	var errs []error
	for i, p := range a.Expanded() {
		if _, err := regexp.Compile(p); err != nil {
			errs = append(errs, fmt.Errorf("attribution.patterns: %q doesn't compile", a.Patterns[i]))
		}
	}
	return errs
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

// SizeBudgets are the most prose words, fenced code excluded, a deliverable
// of each kind runs before check-size sends it back. 0 is no budget.
type SizeBudgets struct {
	Plan    int `yaml:"plan"`
	Report  int `yaml:"report"`
	Scratch int `yaml:"scratch"`
	Post    int `yaml:"post"`
}

// problems names a budget below 0, which would silently mean "off".
func (s SizeBudgets) problems() []error {
	var errs []error
	for _, b := range []struct {
		name string
		n    int
	}{{"plan", s.Plan}, {"report", s.Report}, {"scratch", s.Scratch}, {"post", s.Post}} {
		if b.n < 0 {
			errs = append(errs, fmt.Errorf("size_budgets: %s is %d; a budget is 0 (off) or more", b.name, b.n))
		}
	}
	return errs
}

// WeakenedChecks are the paths and patterns go/internal/weaken reads an
// edit by: the files it skips, the files that are tests, and the lines
// that skip a test, silence a check, or assert.
type WeakenedChecks struct {
	// Block makes a skip, focus, or suppression block until the line carries
	// `kit: allow <reason>`; else it asks, so no marker gets written.
	Block        bool     `yaml:"block"`
	Skip         []string `yaml:"skip"`
	TestFiles    []string `yaml:"test_files"`
	Skips        []string `yaml:"skips"`
	Suppressions []string `yaml:"suppressions"`
	Assertions   []string `yaml:"assertions"`
	// TableLoops are the lines that run a test body once per table row:
	// an assertion moved into one isn't lost.
	TableLoops []string `yaml:"table_loops"`
	// Snapshots are the snapshot files' globs; SnapshotUpdates, the
	// commands that rewrite them.
	Snapshots       []string `yaml:"snapshots"`
	SnapshotUpdates []string `yaml:"snapshot_updates"`
}

// problems names each pattern that doesn't compile.
func (w WeakenedChecks) problems() []error {
	var errs []error
	for _, l := range []struct {
		name     string
		patterns []string
	}{{"skips", w.Skips}, {"suppressions", w.Suppressions}, {"assertions", w.Assertions}, {"table_loops", w.TableLoops}, {"snapshot_updates", w.SnapshotUpdates}} {
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
// applies to (all when empty), the surfaces where a hit only notes though
// its tier blocks (NoteOn), and the rewrite direction a hit reports.
type SlopRule struct {
	ID     string   `yaml:"id"`
	Tier   int      `yaml:"tier"`
	On     []string `yaml:"on"`
	NoteOn []string `yaml:"note_on"`
	Match  string   `yaml:"match"`
	Fix    string   `yaml:"fix"`
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
		for _, on := range append(slices.Clone(r.On), r.NoteOn...) {
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
// explain trigger raises it to Explain. Write is per write-* skill, the whole
// reply. A prompt that starts with an UncappedCommands command has none.
// 0 is no ceiling.
type ReplyLimits struct {
	Modes            map[string]ReplyMode `yaml:"modes"`
	ExplainTriggers  []string             `yaml:"explain_triggers"`
	DetailTriggers   []string             `yaml:"detail_triggers"`
	UncappedCommands []string             `yaml:"uncapped_commands"`
	Write            map[string]int       `yaml:"write"`

	// Mode is the reply_length mode in force, and Chat and Explain its
	// targets; Slack is reply_slack as a percent. load resolves them.
	Mode          string `yaml:"-"`
	Chat, Explain int    `yaml:"-"`
	Slack         int    `yaml:"-"`
}

// ReplyMode is one reply_length mode's word targets, 0 for none.
type ReplyMode struct {
	Chat    int `yaml:"chat"`
	Explain int `yaml:"explain"`
}

// Allowance is how far past target a reply or deliverable may run before
// it's flagged: target plus the slack.
func (l ReplyLimits) Allowance(target int) int { return target + target*l.Slack/100 }

// negatives names each ceiling below 0, which would silently mean "off".
func (l ReplyLimits) negatives() []error {
	var errs []error
	check := func(name string, n int) {
		if n < 0 {
			errs = append(errs, fmt.Errorf("reply_limits: %s is %d; a ceiling is 0 (off) or more", name, n))
		}
	}
	for _, mode := range slices.Sorted(maps.Keys(l.Modes)) {
		check("modes."+mode+".chat", l.Modes[mode].Chat)
		check("modes."+mode+".explain", l.Modes[mode].Explain)
	}
	for _, kind := range slices.Sorted(maps.Keys(l.Write)) {
		check("write."+kind, l.Write[kind])
	}
	return errs
}
