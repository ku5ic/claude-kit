package e2e

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"testing"

	"go.yaml.in/yaml/v3"
)

// go/.golangci.yml enforces the numbers metric-thresholds.md owns, so the
// two can't drift: function size, nesting, file size, and the Go branch line.
func TestLintMatchesTheThresholds(t *testing.T) {
	t.Parallel()
	owner, err := os.ReadFile(filepath.Join(kitRoot, "skills", "engineering-fundamentals", "reference", "metric-thresholds.md"))
	if err != nil {
		t.Fatal(err)
	}
	number := func(pattern string) int {
		t.Helper()
		m := regexp.MustCompile(pattern).FindSubmatch(owner)
		if m == nil {
			t.Fatalf("metric-thresholds.md has no line matching %q", pattern)
		}
		n, _ := strconv.Atoi(string(m[1]))
		return n
	}
	var lint struct {
		Linters struct {
			Settings struct {
				Funlen struct {
					Lines int `yaml:"lines"`
				} `yaml:"funlen"`
				Cyclop struct {
					MaxComplexity int `yaml:"max-complexity"`
				} `yaml:"cyclop"`
				Revive struct {
					Rules []reviveRule `yaml:"rules"`
				} `yaml:"revive"`
			} `yaml:"settings"`
		} `yaml:"linters"`
	}
	data, err := os.ReadFile(filepath.Join(kitRoot, "go", ".golangci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(data, &lint); err != nil {
		t.Fatal(err)
	}
	rule := func(name string) any {
		t.Helper()
		rules := lint.Linters.Settings.Revive.Rules
		i := slices.IndexFunc(rules, func(r reviveRule) bool { return r.Name == name })
		if i < 0 || len(rules[i].Arguments) == 0 {
			t.Fatalf("revive has no %s rule with arguments", name)
		}
		return rules[i].Arguments[0]
	}
	fileLimit, _ := rule("file-length-limit").(map[string]any)
	for _, c := range []struct {
		name      string
		lint, own int
	}{
		{"function size, funlen.lines", lint.Linters.Settings.Funlen.Lines, number(`\| Function size\s+\| over (\d+) lines`)},
		{"nesting, revive max-control-nesting", toInt(rule("max-control-nesting")), number(`\| Nesting\s+\| conditionals nested deeper than (\d+)`)},
		{"file size, revive file-length-limit", toInt(fileLimit["max"]), number(`\| File size\s+\| over (\d+) lines`)},
		{"branches, cyclop.max-complexity", lint.Linters.Settings.Cyclop.MaxComplexity, number("`cyclop` over (\\d+)")},
	} {
		if c.lint != c.own {
			t.Errorf("%s: .golangci.yml says %d, metric-thresholds.md %d", c.name, c.lint, c.own)
		}
	}
}

// reviveRule is one entry of .golangci.yml's revive rules.
type reviveRule struct {
	Name      string `yaml:"name"`
	Arguments []any  `yaml:"arguments"`
}

// toInt is a YAML number decoded into any, or -1.
func toInt(v any) int {
	if n, ok := v.(int); ok {
		return n
	}
	return -1
}
