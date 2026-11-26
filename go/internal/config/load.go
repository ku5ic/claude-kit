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
	clean := filepath.Clean(path)
	return fsx.Within(p.ScratchHome(), clean) ||
		strings.HasSuffix(clean, "/.claude/scratch") || strings.Contains(clean, "/.claude/scratch/") ||
		sessionScratchpad.MatchString(clean)
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
	base, err := readNode(p.Base)
	if err != nil {
		return nil, nil, err
	}
	warnings := validate(p.Base)

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
		// mergeNode rewrote base in place, so it is read again: losing the
		// overlay beats losing every guard's lists.
		if merged, err = readNode(p.Base); err != nil {
			return nil, warnings, err
		}
		tag = "base"
		cfg = Config{}
		if err := merged.Decode(&cfg); err != nil {
			return nil, warnings, fmt.Errorf("decode %s: %w", p.Base, err)
		}
		for _, l := range layers {
			if l.path == p.Overlay || l.path == p.Settings {
				warnings = append(warnings, Warning{l.path, errors.New("ignored, with the preset: the overlay and /config options don't merge with kit.yml, so only kit.yml applies")})
			}
		}
		from = p.Base
	}
	applyDefaults(&cfg)
	cfg.resolveReply()
	if tag == "merged" {
		// mergeNode appends the overlay's list to kit.yml's, after the
		// preset's; the /config options hold no list.
		if i := slices.IndexFunc(layers, func(l layer) bool { return l.path == p.Overlay }); i >= 0 {
			if seq := mappingValue(layers[i].node, "global_installs"); seq != nil && seq.Kind == yaml.SequenceNode {
				cfg.OverlayInstalls = len(seq.Content)
			}
		}
	}
	patterns, bad := cfg.compileInstalls(p)
	cfg.installs = &installPatterns{patterns, bad}
	for _, err := range slices.Concat(cfg.unknownDisables(), cfg.ReplyLimits.negatives(), cfg.Slop.problems(), cfg.Attribution.problems(), cfg.Claims.problems(), cfg.SizeBudgets.problems(), cfg.WeakenedChecks.problems(), cfg.PackageLookups.problems(), cfg.dropEmptyEditCmds(), cfg.pickerProblems()) {
		warnings = append(warnings, Warning{from, err})
	}
	warnings = append(warnings, bad...)
	cfg.Tag = tag
	cfg.From = from
	return &cfg, warnings, nil
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
	base, err := readNode(p.Base)
	if err != nil {
		return nil, err
	}
	layers, _ := overlays(p, base)
	sources := map[string][]string{}
	for _, l := range append([]layer{{p.Base, base}}, layers...) {
		name := filepath.Base(l.path)
		if filepath.Base(filepath.Dir(l.path)) == "presets" {
			name = "presets/" + name
		}
		for i := 0; i+1 < len(l.node.Content); i += 2 {
			key := l.node.Content[i].Value
			sources[key] = append(sources[key], name)
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

// mergeNode merges src into dst in place, as yq's `*+`: mappings merge key by
// key in dst's order with src's new keys after, sequences append, and any
// other pairing takes src's value. Three exceptions: a replacedLists key
// takes src's list, an item with an id (a slop rule) replaces dst's item of
// that id in place, and a "!x" item removes dst's item x. It returns each
// "!x" that removed nothing, as "<key>: !x"; in an exceptionLists list, one
// stays as an exception instead.
func mergeNode(dst, src *yaml.Node) []string { return merge(dst, src, "") }

func merge(dst, src *yaml.Node, key string) []string {
	var unmatched []string
	switch {
	case dst.Kind == yaml.MappingNode && src.Kind == yaml.MappingNode:
		for i := 0; i+1 < len(src.Content); i += 2 {
			key, value := src.Content[i], src.Content[i+1]
			switch existing := mappingValue(dst, key.Value); {
			case existing == nil:
				dst.Content = append(dst.Content, key, value)
			case replacedLists[key.Value]:
				*existing = *value
			default:
				unmatched = append(unmatched, merge(existing, value, key.Value)...)
			}
		}
	case dst.Kind == yaml.SequenceNode && src.Kind == yaml.SequenceNode:
		for _, item := range src.Content {
			if removed, ok := strings.CutPrefix(item.Value, "!"); ok && item.Kind == yaml.ScalarNode {
				before := len(dst.Content)
				dst.Content = slices.DeleteFunc(dst.Content, func(n *yaml.Node) bool { return n.Kind == yaml.ScalarNode && n.Value == removed })
				if len(dst.Content) < before {
					continue
				}
				if !exceptionLists[key] {
					unmatched = append(unmatched, key+": "+item.Value)
					continue
				}
			}
			if i := itemWithID(dst, item); i >= 0 {
				dst.Content[i] = item
				continue
			}
			dst.Content = append(dst.Content, item)
		}
	default:
		*dst = *src
	}
	return unmatched
}

// exceptionLists are the pattern lists where a "!x" that removes no item
// narrows the patterns before it, as sensitive_paths' !.env.example does.
var exceptionLists = map[string]bool{"sensitive_paths": true}

// replacedLists are the lists that are a setting, not a set to add to:
// block_tiers, and the argv lists.
var replacedLists = map[string]bool{"block_tiers": true, "cmd": true, "private_cmd": true, "command": true}

// itemWithID is the index of seq's mapping with item's id, -1 when item has
// no id or seq none with it.
func itemWithID(seq, item *yaml.Node) int {
	if item.Kind != yaml.MappingNode {
		return -1
	}
	id := mappingValue(item, "id")
	if id == nil {
		return -1
	}
	for i, existing := range seq.Content {
		if existing.Kind == yaml.MappingNode {
			if other := mappingValue(existing, "id"); other != nil && other.Value == id.Value {
				return i
			}
		}
	}
	return -1
}

func mappingValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}
