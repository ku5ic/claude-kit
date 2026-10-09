// Package extract reads task names, workspace members, and versions out of
// manifests. Each extractor takes a file and a dotted path (".scripts",
// ".tool.poe.tasks") and returns nothing, not an error, when the file or the
// path is missing: absence is the common case, not a failure.
package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
	"go.yaml.in/yaml/v3"
)

// Run dispatches kit.yml's extractor names. Only names listed here run; any
// other name is an error, never a command.
func Run(name, file, arg string) ([]string, error) {
	switch name {
	case "json_keys":
		return JSONKeys(file, arg), nil
	case "json_array":
		return JSONArray(file, arg), nil
	case "json_value":
		return single(JSONValue(file, arg)), nil
	case "toml_keys":
		return TOMLKeys(file, arg), nil
	case "toml_array":
		return TOMLArray(file, arg), nil
	case "yaml_array":
		return YAMLArray(file, arg), nil
	case "toml_package_version":
		return single(TOMLPackageVersion(file, arg)), nil
	case "regex_lines":
		return RegexLines(file, arg), nil
	case "make_targets":
		return MakeTargets(file), nil
	case "just_recipes":
		return JustRecipes(file), nil
	}
	return nil, fmt.Errorf("unknown extractor in kit.yml: %s", name)
}

func single(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

// splitPath turns ".a.b" into [a b], as jq's getpath with empty parts dropped.
func splitPath(path string) []string {
	var parts []string
	for part := range strings.SplitSeq(strings.TrimPrefix(path, "."), ".") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// GetPath walks decoded JSON/TOML/YAML maps; nil when any step is missing.
func GetPath(value any, path string) any {
	for _, part := range splitPath(path) {
		m, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = m[part]
	}
	return value
}

func strings_(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func readJSON(file string) any {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var value any
	if json.Unmarshal(data, &value) != nil {
		return nil
	}
	return value
}

func readTOML(file string) any {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var value map[string]any
	if toml.Unmarshal(data, &value) != nil {
		return nil
	}
	return value
}

// JSONKeys lists the keys of the object at path, in file order.
func JSONKeys(file, path string) []string {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	raw := json.RawMessage(data)
	for _, part := range splitPath(path) {
		var m map[string]json.RawMessage
		if json.Unmarshal(raw, &m) != nil {
			return nil
		}
		next, ok := m[part]
		if !ok {
			return nil
		}
		raw = next
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil
	}
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil
		}
		keys = append(keys, tok.(string))
		var skip json.RawMessage
		if dec.Decode(&skip) != nil {
			return nil
		}
	}
	return keys
}

// JSONArray lists the string items of the array at path.
func JSONArray(file, path string) []string {
	return strings_(GetPath(readJSON(file), path))
}

// JSONValue is the string or number at path, "" otherwise.
func JSONValue(file, path string) string {
	switch v := GetPath(readJSON(file), path).(type) {
	case string:
		return v
	case float64:
		return strings.TrimSuffix(fmt.Sprintf("%v", v), ".0")
	}
	return ""
}

// TOMLKeys lists the keys of the table at path, in file order: task names
// keep the order the project wrote them in.
func TOMLKeys(file, path string) []string {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	children := map[string][]string{}
	seen := map[string]bool{}
	record := func(full []string) {
		for i := range full {
			parent := strings.Join(full[:i], ".")
			edge := parent + "\x00" + full[i]
			if !seen[edge] {
				seen[edge] = true
				children[parent] = append(children[parent], full[i])
			}
		}
	}
	var walkInline func(prefix []string, node *unstable.Node)
	walkInline = func(prefix []string, node *unstable.Node) {
		it := node.Children()
		for it.Next() {
			kv := it.Node()
			if kv.Kind != unstable.KeyValue {
				continue
			}
			full := append(append([]string{}, prefix...), keyParts(kv)...)
			record(full)
			if kv.Value().Kind == unstable.InlineTable {
				walkInline(full, kv.Value())
			}
		}
	}

	var current []string
	var p unstable.Parser
	p.Reset(data)
	for p.NextExpression() {
		expr := p.Expression()
		switch expr.Kind {
		case unstable.Table, unstable.ArrayTable:
			current = keyParts(expr)
			record(current)
		case unstable.KeyValue:
			full := append(append([]string{}, current...), keyParts(expr)...)
			record(full)
			if expr.Value().Kind == unstable.InlineTable {
				walkInline(full, expr.Value())
			}
		}
	}
	if p.Error() != nil {
		return nil
	}
	// Keys only of a table: a scalar at the path has no children to list.
	if _, isTable := GetPath(readTOML(file), path).(map[string]any); !isTable {
		return nil
	}
	return children[strings.Join(splitPath(path), ".")]
}

func keyParts(node *unstable.Node) []string {
	var parts []string
	it := node.Key()
	for it.Next() {
		parts = append(parts, string(it.Node().Data))
	}
	return parts
}

// TOMLArray lists the string items of the array at path.
func TOMLArray(file, path string) []string {
	return strings_(GetPath(readTOML(file), path))
}

// TOMLString is the string at path, "" otherwise.
func TOMLString(file, path string) string {
	s, _ := GetPath(readTOML(file), path).(string)
	return s
}

// TOMLHas is true when path exists, even as an empty table: a bare
// [tool.ruff] means "use ruff with defaults".
func TOMLHas(file, path string) bool {
	return GetPath(readTOML(file), path) != nil
}

// TOMLPackageVersion is the version of [[package]] name in a uv.lock or
// poetry.lock, matched case-insensitively as pip names are.
func TOMLPackageVersion(file, name string) string {
	packages, _ := GetPath(readTOML(file), ".package").([]any)
	for _, pkg := range packages {
		m, ok := pkg.(map[string]any)
		if !ok {
			continue
		}
		if pkgName, _ := m["name"].(string); strings.EqualFold(pkgName, name) {
			version, _ := m["version"].(string)
			return version
		}
	}
	return ""
}

// YAMLArray lists the string items of the sequence at path.
func YAMLArray(file, path string) []string {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var value any
	if yaml.Unmarshal(data, &value) != nil {
		return nil
	}
	return strings_(GetPath(value, path))
}

var (
	makeTarget  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*[[:space:]]*:([^=]|$)`)
	justRecipe  = regexp.MustCompile(`^@?[A-Za-z_][A-Za-z0-9_-]*([[:space:]][^:=]*)?:([^=]|$)`)
	justName    = regexp.MustCompile(`^@?([A-Za-z0-9_-]+)`)
	makeNameEnd = regexp.MustCompile(`[[:space:]]*:.*`)
)

// MakeTargets lists explicit targets in file order, skipping special
// (.PHONY), pattern (%), and variable-assignment (:=) lines.
func MakeTargets(file string) []string {
	return matchLines(file, makeTarget, func(line string) string {
		return makeNameEnd.ReplaceAllString(line, "")
	})
}

// JustRecipes lists recipe names: `just --summary` when just is installed,
// else recipe header lines.
func JustRecipes(file string) []string {
	if _, err := os.Stat(file); err != nil {
		return nil
	}
	if _, err := exec.LookPath("just"); err == nil {
		out, err := exec.Command("just", "--justfile", file, "--summary").Output()
		if err != nil {
			return nil
		}
		return strings.Fields(string(out))
	}
	return matchLines(file, justRecipe, func(line string) string {
		return justName.FindStringSubmatch(line)[1]
	})
}

// RegexLines returns, for each matching line, its last capture group.
func RegexLines(file, pattern string) []string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	var out []string
	EachLine(file, func(line string) {
		if m := re.FindStringSubmatch(line); m != nil {
			out = append(out, m[len(m)-1])
		}
	})
	return out
}

func matchLines(file string, re *regexp.Regexp, name func(string) string) []string {
	var out []string
	seen := map[string]bool{}
	EachLine(file, func(line string) {
		if !re.MatchString(line) {
			return
		}
		if n := name(line); !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	})
	return out
}

// EachLine calls fn for each line of file, or never when the file can't be
// read, so a caller never acts on half a file.
func EachLine(file string, fn func(string)) {
	data, err := os.ReadFile(file)
	if err != nil {
		return
	}
	for line := range strings.Lines(string(data)) {
		fn(strings.TrimRight(line, "\r\n"))
	}
}
