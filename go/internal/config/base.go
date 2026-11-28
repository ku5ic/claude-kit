package config

import (
	"fmt"
	"path/filepath"
	"slices"

	"go.yaml.in/yaml/v3"
)

// BaseDir is kit.d/ beside Base: the topic files merged into kit.yml.
func (p Paths) BaseDir() string { return filepath.Join(filepath.Dir(p.Base), "kit.d") }

// BaseFiles are kit.yml, then each kit.d/*.yml in name order.
func (p Paths) BaseFiles() []string {
	topics, _ := filepath.Glob(filepath.Join(p.BaseDir(), "*.yml"))
	slices.Sort(topics)
	return append([]string{p.Base}, topics...)
}

// readBase is kit.yml with every kit.d/ topic file merged in, each file's
// own keys as a layer, and each file's strict-decode warnings. A top-level
// key two base files set is a warning too; the second merges over the first.
func readBase(p Paths) (*yaml.Node, []layer, []Warning, error) {
	var base *yaml.Node
	var parts []layer
	var warnings []Warning
	owner := map[string]string{}
	for _, path := range p.BaseFiles() {
		node, err := readNode(path)
		if err != nil {
			return nil, nil, warnings, err
		}
		warnings = append(warnings, validate(path)...)
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			if first, ok := owner[key]; ok {
				warnings = append(warnings, Warning{path, fmt.Errorf("%s: %s sets it too; keep each key in one base file", key, baseName(first))})
			}
			owner[key] = path
		}
		// Each layer keeps its own key list, since merging grows kit.yml's node.
		parts = append(parts, layer{path, &yaml.Node{Kind: node.Kind, Content: slices.Clone(node.Content)}})
		if base == nil {
			base = node
			continue
		}
		mergeNode(base, node)
	}
	return base, parts, warnings, nil
}

// baseName is path as `kit config` names a file: kit.yml, or a kit.d/ or
// presets/ file with its folder.
func baseName(path string) string {
	name := filepath.Base(path)
	if dir := filepath.Base(filepath.Dir(path)); dir == "kit.d" || dir == "presets" {
		return dir + "/" + name
	}
	return name
}
