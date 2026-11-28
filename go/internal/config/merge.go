package config

import (
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// mergeNode merges src into dst in place, as yq's `*+`: mappings merge key by
// key in dst's order with src's new keys after, sequences append, and any
// other pairing takes src's value. Three exceptions: a replacedLists key
// takes src's list, an item with an id (a slop rule) replaces dst's item of
// that id in place, and a "!x" item removes dst's item x. It returns each
// "!x" that removed nothing, as "<key>: !x"; in an exceptionLists list, one
// stays as an exception instead.
func mergeNode(dst, src *yaml.Node) []string { return merge(dst, src, "") }

func merge(dst, src *yaml.Node, key string) []string {
	switch {
	case dst.Kind == yaml.MappingNode && src.Kind == yaml.MappingNode:
		return mergeMapping(dst, src)
	case dst.Kind == yaml.SequenceNode && src.Kind == yaml.SequenceNode:
		return mergeSequence(dst, src, key)
	}
	*dst = *src
	return nil
}

// mergeMapping merges mapping src into dst key by key.
func mergeMapping(dst, src *yaml.Node) []string {
	var unmatched []string
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
	return unmatched
}

// mergeSequence merges sequence src, key's list, into dst: a "!x" removes
// x, an item with an id replaces dst's, and any other item appends.
func mergeSequence(dst, src *yaml.Node, key string) []string {
	var unmatched []string
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
