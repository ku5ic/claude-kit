package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
)

// Paths are the kit's fixed locations, resolved by ResolvePaths.
type Paths struct {
	Root      string // kit root: $CLAUDE_PLUGIN_ROOT, else the parent of the binary's bin/
	ClaudeDir string // Claude Code's config dir: $CLAUDE_CONFIG_DIR, else ~/.claude
	Base      string // Root/kit.yml
	Overlay   string // ClaudeDir/claude-kit.local.yml
	Settings  string // ClaudeDir/settings.json, whose pluginConfigs hold the /config options
	Manifest  string // Root/.claude-plugin/plugin.json, which declares them
}

func (p Paths) LogDir() string      { return filepath.Join(p.ClaudeDir, "logs") }
func (p Paths) CacheDir() string    { return filepath.Join(p.ClaudeDir, "cache") }
func (p Paths) ScratchHome() string { return filepath.Join(p.ClaudeDir, "scratch") }
func (p Paths) PlansHome() string   { return filepath.Join(p.ClaudeDir, "plans") }

// IsScratch is true for a path in, or at, a project's .claude/scratch, the
// home scratch, which $CLAUDE_CONFIG_DIR can move, or this user's Claude
// Code session scratchpad. Any other directory named scratch is project
// code and gets every check.
func (p Paths) IsScratch(path string) bool {
	_, ok := p.ScratchRoot(path)
	return ok
}

// ScratchRoot is the scratch directory path is in or at, as IsScratch finds
// it, written as path writes it.
func (p Paths) ScratchRoot(path string) (string, bool) {
	clean := filepath.Clean(path)
	if fsx.Within(p.ScratchHome(), clean) {
		return p.ScratchHome(), true
	}
	if i := strings.Index(clean+"/", "/.claude/scratch/"); i >= 0 {
		return clean[:i+len("/.claude/scratch")], true
	}
	if m := sessionScratchpad.FindString(clean); m != "" {
		return strings.TrimSuffix(m, "/"), true
	}
	return "", false
}

// sessionScratchpad is the throwaway folder Claude Code gives each session,
// /tmp/claude-<uid>/<project>/<session>/scratchpad, as macOS's /private/tmp
// too.
var sessionScratchpad = regexp.MustCompile(fmt.Sprintf(`^(?:/private)?/tmp/claude-%d/[^/]+/[^/]+/scratchpad(?:/|$)`, os.Getuid()))

// LogFile is the JSONL log called name.
func (p Paths) LogFile(name string) string { return filepath.Join(p.LogDir(), name+".jsonl") }

// ScratchRegistry lists every project scratch dir, for scratch-rotate.
func (p Paths) ScratchRegistry() string { return filepath.Join(p.LogDir(), "scratch-registry.txt") }

// SessionFile is cache.SessionPath under p's cache dir.
func (p Paths) SessionFile(kind, session string, parts ...string) string {
	return cache.SessionPath(p.CacheDir(), kind, session, parts...)
}

// ResolvePaths reads the environment. exe is the running binary's path,
// used only when CLAUDE_PLUGIN_ROOT is unset.
func ResolvePaths(exe string) (Paths, error) {
	root := os.Getenv("CLAUDE_PLUGIN_ROOT")
	if root == "" {
		return ResolveCheckout(exe)
	}
	return pathsAt(root)
}

// ResolveCheckout is ResolvePaths rooted at the tree holding exe, for what
// must read the checkout: a hook's process has CLAUDE_PLUGIN_ROOT set to the
// installed plugin.
func ResolveCheckout(exe string) (Paths, error) {
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve kit root from %s: %w", exe, err)
	}
	return pathsAt(filepath.Dir(filepath.Dir(real)))
}

func pathsAt(root string) (Paths, error) {
	claudeDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if claudeDir == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, fmt.Errorf("resolve home: %w", err)
		}
		claudeDir = filepath.Join(userHome, ".claude")
	}
	return Paths{
		Root:      root,
		ClaudeDir: claudeDir,
		Base:      filepath.Join(root, "kit.yml"),
		Overlay:   filepath.Join(claudeDir, "claude-kit.local.yml"),
		Settings:  filepath.Join(claudeDir, "settings.json"),
		Manifest:  filepath.Join(root, ".claude-plugin", "plugin.json"),
	}, nil
}

// Warning is a problem in one config file that doesn't stop loading, such as
// an unknown key.
type Warning struct {
	File string
	Err  error
}

func (w Warning) String() string { return w.File + ": " + w.Err.Error() }

// Load reads kit.yml and, when present, the preset the overlay names and
// then the overlay, each merged over the last: maps
// merge and sequences append, and a "!x" item removes x (mergeNode names
// the exceptions).
// Each file is also decoded strictly on its own, and what that rejects comes
// back as warnings naming the file. The result is cached on disk until
// either file changes, since every hook call would otherwise pay the parse.
func Load(p Paths) (*Config, []Warning, error) {
	file, key := p.ConfigCache(), cacheKey(p)
	if file == "" || key == "" {
		return load(p)
	}
	if cfg, warnings, ok := loadCached(file, key); ok {
		return cfg, warnings, nil
	}
	cfg, warnings, err := load(p)
	if err == nil {
		storeCached(file, key, cfg, warnings)
	}
	return cfg, warnings, err
}

// LoadBase reads kit.yml alone, for what must not vary by machine.
func LoadBase(p Paths) (*Config, []Warning, error) {
	p.Overlay, p.Settings = "", ""
	return load(p)
}

func load(p Paths) (*Config, []Warning, error) {
	base, parts, warnings, err := readBase(p)
	if err != nil {
		return nil, warnings, err
	}

	merged := base
	tag := "base"
	layers, layerWarnings := overlays(p, base)
	warnings = append(warnings, layerWarnings...)
	from := p.Base
	if len(layers) > 0 {
		from = userFile(p, layers)
	}
	for _, layer := range layers {
		for _, item := range mergeNode(merged, layer.node) {
			warnings = append(warnings, Warning{layer.path, fmt.Errorf("%s removes nothing: no earlier list holds it", item)})
		}
		tag = "merged"
	}

	var cfg Config
	if err := merged.Decode(&cfg); err != nil {
		if tag != "merged" {
			return nil, warnings, fmt.Errorf("decode %s: %w", p.Base, err)
		}
		var ignored []Warning
		if cfg, ignored, err = decodeBaseOnly(p, layers); err != nil {
			return nil, warnings, err
		}
		warnings, tag, from = append(warnings, ignored...), "base", p.Base
	}
	applyDefaults(&cfg)
	cfg.resolveReply()
	if tag == "merged" {
		cfg.OverlayInstalls = overlayInstalls(p, layers)
	}
	patterns, bad := cfg.compileInstalls(p)
	cfg.installs = &installPatterns{patterns, bad}
	for _, err := range cfg.keyProblems() {
		file := from
		if from == p.Base {
			file = baseOwner(parts, err)
		}
		warnings = append(warnings, Warning{file, err})
	}
	warnings = append(warnings, bad...)
	cfg.Tag = tag
	cfg.From = from
	return &cfg, warnings, nil
}

// decodeBaseOnly is kit.yml and kit.d/ decoded alone, when the merged
// layers don't decode: mergeNode rewrote base in place, so it is read again, since
// losing the overlay beats losing every guard's lists. Each user layer it
// leaves out gets a warning.
func decodeBaseOnly(p Paths, layers []layer) (Config, []Warning, error) {
	var cfg Config
	base, _, _, err := readBase(p)
	if err != nil {
		return cfg, nil, err
	}
	if err := base.Decode(&cfg); err != nil {
		return cfg, nil, fmt.Errorf("decode %s: %w", p.Base, err)
	}
	var warnings []Warning
	for _, l := range layers {
		if l.path == p.Overlay || l.path == p.Settings {
			warnings = append(warnings, Warning{l.path, errors.New("ignored, with the preset: the overlay and /config options don't merge with kit.yml, so only kit.yml applies")})
		}
	}
	return cfg, warnings, nil
}

// overlayInstalls is how many global_installs entries the user's overlay
// adds: mergeNode appends its list to kit.yml's, after the preset's, and
// the /config options hold no list.
func overlayInstalls(p Paths, layers []layer) int {
	i := slices.IndexFunc(layers, func(l layer) bool { return l.path == p.Overlay })
	if i < 0 {
		return 0
	}
	if seq := mappingValue(layers[i].node, "global_installs"); seq != nil && seq.Kind == yaml.SequenceNode {
		return len(seq.Content)
	}
	return 0
}

// keyProblems is every key's own validation problem.
func (c *Config) keyProblems() []error {
	return slices.Concat(c.unknownDisables(), c.ReplyLimits.negatives(), c.Slop.problems(), c.Attribution.problems(), c.Claims.problems(),
		c.SizeBudgets.problems(), c.WeakenedChecks.problems(), c.PackageLookups.problems(), c.dropEmptyEditCmds(), c.pickerProblems())
}

// userFile is the file a merged config's warnings name: the overlay when
// it's among layers, else settings.json, whose /config options pick a preset
// or set a key.
func userFile(p Paths, layers []layer) string {
	if slices.ContainsFunc(layers, func(l layer) bool { return l.path == p.Overlay }) {
		return p.Overlay
	}
	return p.Settings
}

// layer is one file merged over kit.yml.
type layer struct {
	path string
	node *yaml.Node
}

// overlays are what's merged over kit.yml, in order: the preset, then the
// overlay, then the /config options (pluginOptions), so /config wins. The
// preset is the one /config picks, else the overlay's. A file that's missing
// adds nothing, and one that can't be read is a warning.
func overlays(p Paths, base *yaml.Node) ([]layer, []Warning) {
	var warnings []Warning
	overlay, err := readNode(p.Overlay)
	switch {
	case err == nil:
		warnings = validate(p.Overlay)
	case !errors.Is(err, os.ErrNotExist):
		warnings = append(warnings, Warning{p.Overlay, fmt.Errorf("ignored: %w", err)})
	}
	options, optionWarnings := pluginOptions(p, base)
	warnings = append(warnings, optionWarnings...)

	var out []layer
	for _, l := range []layer{{p.Settings, options}, {p.Overlay, overlay}} {
		if l.node == nil {
			continue
		}
		if name := mappingValue(l.node, "preset"); name != nil && name.Value != "" {
			preset, presetWarnings := presetLayer(p, l.path, name.Value)
			warnings = append(warnings, presetWarnings...)
			out = append(out, preset...)
			break
		}
	}
	for _, l := range []layer{{p.Overlay, overlay}, {p.Settings, options}} {
		if l.node != nil {
			out = append(out, l)
		}
	}
	return out, warnings
}

// presetLayer is the preset called name, which the file from picked.
func presetLayer(p Paths, from, name string) ([]layer, []Warning) {
	path := p.presetFile(name)
	preset, err := readNode(path)
	switch {
	case err == nil:
		return []layer{{path, preset}}, validate(path)
	case path == "" || errors.Is(err, os.ErrNotExist):
		return nil, []Warning{{from, fmt.Errorf("preset: no preset %q under %s", name, filepath.Join(p.Root, "presets"))}}
	default:
		return nil, []Warning{{path, fmt.Errorf("ignored: %w", err)}}
	}
}

// presetName is what a preset may be called: a file name, never a path.
var presetName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// presetFile is <root>/presets/<name>.yml, "" for a name that isn't one.
func (p Paths) presetFile(name string) string {
	if !presetName.MatchString(name) {
		return ""
	}
	return filepath.Join(p.Root, "presets", name+".yml")
}

// Merged returns the effective merged document, for `kit config`: each
// top-level key carries a line comment naming the files that set it, and a
// picker key shows the value in force.
func Merged(p Paths) (*yaml.Node, error) {
	base, parts, _, err := readBase(p)
	if err != nil {
		return nil, err
	}
	layers, _ := overlays(p, base)
	sources := map[string][]string{}
	for _, l := range append(parts, layers...) {
		for i := 0; i+1 < len(l.node.Content); i += 2 {
			key := l.node.Content[i].Value
			sources[key] = append(sources[key], baseName(l.path))
		}
	}
	for _, l := range layers {
		mergeNode(base, l.node)
	}
	var cfg Config
	if base.Decode(&cfg) == nil {
		for _, p := range cfg.pickers() {
			if value := mappingValue(base, p.key); value != nil && p.value != p.pick() {
				value.Value = p.pick()
			}
		}
	}
	for i := 0; i+1 < len(base.Content); i += 2 {
		key, value := base.Content[i], base.Content[i+1]
		// The encoder drops a key's line comment before a flow-style value.
		target := key
		if value.Style&yaml.FlowStyle != 0 {
			target = value
		}
		target.LineComment = strings.Join(sources[key.Value], ", ")
	}
	return base, nil
}

// ReceiptOnFailures is true when kit.yml's receipt hides a clean pass.
func (c *Config) ReceiptOnFailures() bool { return c.Receipt == "failures" }

// StepPauses are step_pause's values: stop after each plan step, after the
// whole plan, or never. Receipts are receipt's, and ReplySlacks reply_slack's
// with their percent. Exported for the test that holds plugin.json's /config
// choices to them.
var (
	StepPauses  = []string{"step", "plan", "off"}
	Receipts    = []string{"all", "failures"}
	ReplySlacks = map[string]int{"none": 0, "25%": 25, "50%": 50, "100%": 100}
)

// picker is a key whose value is one of a list, and what applies when it's
// unset or not in the list.
type picker struct {
	key, value, fallback string
	values               []string
}

// pick is the value in force.
func (p picker) pick() string {
	if slices.Contains(p.values, p.value) {
		return p.value
	}
	return p.fallback
}

func (c *Config) pickers() []picker {
	return []picker{
		{"step_pause", c.StepPause, "step", StepPauses},
		{"receipt", c.Receipt, "all", Receipts},
		{"reply_length", c.ReplyLength, "terse", slices.Sorted(maps.Keys(c.ReplyLimits.Modes))},
		{"reply_slack", c.ReplySlack, "50%", slices.Sorted(maps.Keys(ReplySlacks))},
	}
}

// picked is the value in force for the picker key.
func (c *Config) picked(key string) string {
	pickers := c.pickers()
	return pickers[slices.IndexFunc(pickers, func(p picker) bool { return p.key == key })].pick()
}

// Pause is step_pause, "step" when it's unset or not one of StepPauses.
func (c *Config) Pause() string { return c.picked("step_pause") }

// pickerProblems is a warning for each picker key holding a value outside
// its list.
func (c *Config) pickerProblems() []error {
	var errs []error
	for _, p := range c.pickers() {
		if p.value != "" && !slices.Contains(p.values, p.value) {
			errs = append(errs, fmt.Errorf("%s: %q isn't one of %s, so %s applies", p.key, p.value, strings.Join(p.values, ", "), p.fallback))
		}
	}
	return errs
}

// resolveReply sets the reply_length mode's targets and the slack.
func (c *Config) resolveReply() {
	l := &c.ReplyLimits
	l.Mode = c.picked("reply_length")
	l.Chat, l.Explain = l.Modes[l.Mode].Chat, l.Modes[l.Mode].Explain
	l.Slack = ReplySlacks[c.picked("reply_slack")]
}

func applyDefaults(cfg *Config) {
	if cfg.LogMaxLines == 0 {
		cfg.LogMaxLines = 10000
	}
	if cfg.GateTimeout == 0 {
		cfg.GateTimeout = 90
	}
}

// readNode returns the top-level mapping of a YAML file.
func readNode(path string) (*yaml.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(doc.Content) == 0 {
		return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("parse %s: top level is not a mapping", path)
	}
	return root, nil
}

func validate(path string) []Warning {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var cfg Config
	err = dec.Decode(&cfg)
	var typeErr *yaml.TypeError
	switch {
	case err == nil, errors.Is(err, io.EOF): // io.EOF: empty or only comments
		return nil
	case errors.As(err, &typeErr):
		warnings := make([]Warning, 0, len(typeErr.Errors))
		for _, msg := range typeErr.Errors {
			warnings = append(warnings, Warning{path, errors.New(msg)})
		}
		return warnings
	default:
		return []Warning{{path, err}}
	}
}
