// Package config loads kit.yml and the user's overlay into one typed Config.
package config

import (
	"fmt"
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
	ShadowRules       []string          `yaml:"shadow_rules"`

	GateDiscovery         GateDiscovery `yaml:"gate_discovery"`
	DenyFlags             []string      `yaml:"deny_flags"`
	GateEnv               []string      `yaml:"gate_env"`
	GateTimeout           int           `yaml:"gate_timeout"`
	ArchitectureDir       string        `yaml:"architecture_dir"`
	Receipt               string        `yaml:"receipt"`
	StepPause             string        `yaml:"step_pause"`
	GuardSkills           bool          `yaml:"guard_skills"`
	SanitizeTypography    bool          `yaml:"sanitize_typography"`
	Preset                string        `yaml:"preset"`
	UserPins              []string      `yaml:"user_pins"`
	DisabledTaskProviders []string      `yaml:"disabled_task_providers"`
	DisabledChecks        []string      `yaml:"disabled_checks"`

	ReplyLength    string         `yaml:"reply_length"`
	ReplySlack     string         `yaml:"reply_slack"`
	ReplyLimits    ReplyLimits    `yaml:"reply_limits"`
	SanitizeSkip   []string       `yaml:"sanitize_skip"`
	SkipDirs       []string       `yaml:"skip_dirs"`
	Tools          []string       `yaml:"tools"`
	LockfileGlobs  []string       `yaml:"lockfile_globs"`
	Classifier     []string       `yaml:"classifier"`
	OnEdit         []EditRule     `yaml:"on_edit"`
	Slop           Slop           `yaml:"slop"`
	Connectors     Connectors     `yaml:"connectors"`
	Attribution    Attribution    `yaml:"attribution"`
	Claims         Claims         `yaml:"claims"`
	SizeBudgets    SizeBudgets    `yaml:"size_budgets"`
	WeakenedChecks WeakenedChecks `yaml:"weakened_checks"`
	PackageLookups PackageLookups `yaml:"package_lookups"`
	ChatBundle     ChatBundle     `yaml:"chat_bundle"`

	// Tag is "merged" when an overlay or /config options were merged in,
	// else "base". Caches derived from the config carry it in their file
	// name, so deleting the overlay can't leave its derived state in use.
	Tag string `yaml:"-"`

	// From is the file a problem in the merged values is reported against:
	// kit.yml, the overlay, or settings.json. Exported so the on-disk cache
	// keeps it.
	From string `yaml:"-"`

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
	"adr-nudge", "ai-attribution", "aws-ec2-terminate", "aws-s3-force-rb", "aws-s3-recursive-rm",
	"bash-edits", "bash-reports", "checks-failed", "chmod-777", "chmod-broad-x", "citations", "claims", "commit-msg", "commit-wall-of-text",
	"device-write", "disk-tool", "docker-prune-all-force", "download-to-repo",
	"find-delete", "find-exec-rm", "fork-bomb",
	"gcloud-delete", "git-config-global", "git-dir-edit", "git-force-push", "git-no-verify",
	"git-push-protected", "git-reset-hard", "git-tree-discard", "global-installs-invalid",
	"interpreter-c-wrap", "keychain-delete", "kubectl-delete", "lockfile-edit",
	"pipe-to-shell", "pkg-global-install", "pkg-missing", "pkg-undeclared", "plan-changes", "plan-critic", "plan-done",
	"pm-mismatch", "psql-destructive", "rc-edit", "rc-inplace-edit", "rc-redirect", "redis-destructive",
	"review-checks", "review-pick", "review-report", "rm-recursive", "rules-gate", "scratch-rotate", "sensitive-read", "sensitive-write",
	"size", "skills-gate", "skills-nudge", "slop", "slop-vocabulary", "staged-secret", "terraform-auto-approve", "terraform-destroy",
	"ui-undefined-token", "unread-text", "weaken", "xargs-rm",
}

// AskRules are the slugs only an ask logs, for `kit stats --unused`. An ask
// can't be disabled, so disabled_rules can't name them. TestRulesMatchTheCode
// holds it to the code's asks.
var AskRules = []string{
	"ci-workflow", "commit-approval", "computed-command", "download-target", "gate-config",
	"git-add-credential", "git-clean", "git-history-loss", "git-push", "git-ref-delete",
	"kit-git-base", "kit-switch", "loose-write", "lost-assertions", "network-sink", "package-add",
	"root-artifact", "rsync-delete", "snapshot-rewrite", "trap-unplaced", "unparsed", "zsh-expansion",
}

// unknownDisables names the disabled_checks, disabled_rules, and shadow_rules
// entries that
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
	for _, l := range []struct {
		key   string
		rules []string
	}{{"disabled_rules", c.DisabledRules}, {"shadow_rules", c.ShadowRules}} {
		for _, d := range l.rules {
			id, slop := strings.CutPrefix(d, "slop-")
			known := slices.Contains(Rules, d) || slop && slices.ContainsFunc(c.Slop.Rules, func(r SlopRule) bool { return r.ID == id })
			switch {
			case !known:
				errs = append(errs, fmt.Errorf("%s: %q is no rule; kit explain prints a block's rule", l.key, d))
			case slop && l.key == "shadow_rules":
				errs = append(errs, fmt.Errorf("%s: %q shadows nothing; the slop guards block as one rule, slop", l.key, d))
			}
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
