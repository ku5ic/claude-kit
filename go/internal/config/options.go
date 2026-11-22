package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// pluginOptions is the /config layer: the options plugin.json's userConfig
// declares, as Claude Code saved them under pluginConfigs in the user's
// settings.json, for each one that is preset or a top-level key of base and
// isn't at its manifest default. nil when nothing applies. Managed settings
// aren't read: the kit has no team policy layer.
func pluginOptions(p Paths, base *yaml.Node) (*yaml.Node, []Warning) {
	declared, name := manifestOptions(p.Manifest)
	if name == "" || p.Settings == "" {
		return nil, nil
	}
	data, err := os.ReadFile(p.Settings)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	var settings struct {
		PluginConfigs map[string]struct {
			Options map[string]any `json:"options"`
		} `json:"pluginConfigs"`
	}
	if err == nil {
		err = json.Unmarshal(data, &settings)
	}
	if err != nil {
		return nil, []Warning{{p.Settings, fmt.Errorf("ignored: %w", err)}}
	}
	var ids []string
	for id := range settings.PluginConfigs {
		if strings.HasPrefix(id, name+"@") {
			ids = append(ids, id)
		}
	}
	if len(ids) > 1 {
		slices.Sort(ids)
		return nil, []Warning{{p.Settings, fmt.Errorf("pluginConfigs: %s all name this plugin, so none applies", strings.Join(ids, ", "))}}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	options := settings.PluginConfigs[ids[0]].Options
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, key := range slices.Sorted(maps.Keys(options)) {
		def, ok := declared[key]
		switch {
		case !ok, reflect.DeepEqual(options[key], def):
			continue
		case key != "preset" && mappingValue(base, key) == nil:
			continue
		}
		var value yaml.Node
		if err := value.Encode(options[key]); err != nil {
			return nil, []Warning{{p.Settings, fmt.Errorf("pluginConfigs %s: %s: %w", ids[0], key, err)}}
		}
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &value)
	}
	if len(node.Content) == 0 {
		return nil, nil
	}
	return node, nil
}

// manifestOptions is each userConfig option's default, and the plugin's
// name; "" when the manifest can't be read.
func manifestOptions(path string) (map[string]any, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ""
	}
	var manifest struct {
		Name       string `json:"name"`
		UserConfig map[string]struct {
			Default any `json:"default"`
		} `json:"userConfig"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return nil, ""
	}
	defaults := make(map[string]any, len(manifest.UserConfig))
	for key, option := range manifest.UserConfig {
		defaults[key] = option.Default
	}
	return defaults, manifest.Name
}
