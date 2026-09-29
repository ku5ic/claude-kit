package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// Paths are the kit's fixed locations, as bin/_lib.sh derives them.
type Paths struct {
	Root    string // kit root: $CLAUDE_PLUGIN_ROOT, else the parent of the binary's bin/
	Home    string // Claude Code's config dir: $CLAUDE_CONFIG_DIR, else ~/.claude
	Base    string // Root/kit.yml
	Overlay string // Home/claude-kit.local.yml
}

func (p Paths) LogDir() string      { return filepath.Join(p.Home, "logs") }
func (p Paths) CacheDir() string    { return filepath.Join(p.Home, "cache") }
func (p Paths) ScratchHome() string { return filepath.Join(p.Home, "scratch") }
func (p Paths) PlansHome() string   { return filepath.Join(p.Home, "plans") }

// ResolvePaths reads the environment. exe is the running binary's path,
// used only when CLAUDE_PLUGIN_ROOT is unset.
func ResolvePaths(exe string) (Paths, error) {
	root := os.Getenv("CLAUDE_PLUGIN_ROOT")
	if root == "" {
		real, err := filepath.EvalSymlinks(exe)
		if err != nil {
			return Paths{}, fmt.Errorf("resolve kit root from %s: %w", exe, err)
		}
		root = filepath.Dir(filepath.Dir(real))
	}
	home := os.Getenv("CLAUDE_CONFIG_DIR")
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, fmt.Errorf("resolve home: %w", err)
		}
		home = filepath.Join(userHome, ".claude")
	}
	return Paths{
		Root:    root,
		Home:    home,
		Base:    filepath.Join(root, "kit.yml"),
		Overlay: filepath.Join(home, "claude-kit.local.yml"),
	}, nil
}

// Warning is a problem in one config file that doesn't stop loading, such as
// an unknown key.
type Warning struct {
	File string
	Err  error
}

func (w Warning) String() string { return w.File + ": " + w.Err.Error() }

// Load reads kit.yml and, when present, the overlay merged over it: maps
// merge and sequences append, so an overlay can add but never remove.
// Each file is also decoded strictly on its own, and what that rejects comes
// back as warnings naming the file.
func Load(p Paths) (*Config, []Warning, error) {
	base, err := readNode(p.Base)
	if err != nil {
		return nil, nil, err
	}
	warnings := validate(p.Base)

	merged := base
	tag := "base"
	overlay, err := readNode(p.Overlay)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		warnings = append(warnings, Warning{p.Overlay, fmt.Errorf("ignored: %w", err)})
	default:
		warnings = append(warnings, validate(p.Overlay)...)
		mergeNode(merged, overlay)
		tag = "merged"
	}

	var cfg Config
	if err := merged.Decode(&cfg); err != nil {
		return nil, warnings, fmt.Errorf("decode %s: %w", p.Base, err)
	}
	applyDefaults(&cfg)
	cfg.StackOrder = mappingKeys(mappingValue(merged, "stacks"))
	cfg.VersionOrder = mappingKeys(mappingValue(merged, "versions"))
	cfg.Tag = tag
	return &cfg, warnings, nil
}

func mappingKeys(m *yaml.Node) []string {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	keys := make([]string, 0, len(m.Content)/2)
	for i := 0; i+1 < len(m.Content); i += 2 {
		keys = append(keys, m.Content[i].Value)
	}
	return keys
}

// Merged returns the effective merged document, for `kit config`.
func Merged(p Paths) (*yaml.Node, error) {
	base, err := readNode(p.Base)
	if err != nil {
		return nil, err
	}
	if overlay, err := readNode(p.Overlay); err == nil {
		mergeNode(base, overlay)
	}
	return base, nil
}

func applyDefaults(cfg *Config) {
	if cfg.LogMaxLines == 0 {
		cfg.LogMaxLines = 10000
	}
	if cfg.SubprojectMaxDepth == 0 {
		cfg.SubprojectMaxDepth = 4
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
// other pairing takes src's value.
func mergeNode(dst, src *yaml.Node) {
	switch {
	case dst.Kind == yaml.MappingNode && src.Kind == yaml.MappingNode:
		for i := 0; i+1 < len(src.Content); i += 2 {
			key, value := src.Content[i], src.Content[i+1]
			if existing := mappingValue(dst, key.Value); existing != nil {
				mergeNode(existing, value)
				continue
			}
			dst.Content = append(dst.Content, key, value)
		}
	case dst.Kind == yaml.SequenceNode && src.Kind == yaml.SequenceNode:
		dst.Content = append(dst.Content, src.Content...)
	default:
		*dst = *src
	}
}

func mappingValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}
