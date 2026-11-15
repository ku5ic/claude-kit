package md

import (
	"slices"
	"testing"
)

func kinds(text string) []Kind {
	var out []Kind
	for l := range Lines(text) {
		out = append(out, l.Kind)
	}
	return out
}

func TestLines(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		text string
		want []Kind
	}{
		"frontmatter, then prose":                            {"---\na: b\n---\ntext\n", []Kind{Front, Front, Front, Prose}},
		"a rule later isn't frontmatter":                     {"text\n---\n", []Kind{Prose, Prose}},
		"an unclosed --- isn't frontmatter":                  {"---\n```\nx\n", []Kind{Prose, FenceOpen, Code}},
		"an indented fence":                                  {"  ```go\nx\n  ```\n", []Kind{FenceOpen, Code, FenceClose}},
		"a fence closes only on its own marker":              {"~~~\n```\n~~~\nafter\n", []Kind{FenceOpen, Code, FenceClose, Prose}},
		"a fence never closed runs to the end":               {"```\na\nb", []Kind{FenceOpen, Code, Code}},
		"a shorter fence inside a longer one is code":        {"````md\n```go\nx\n```\n````\nafter\n", []Kind{FenceOpen, Code, Code, Code, FenceClose, Prose}},
		"a longer fence closes a shorter one":                {"```\nx\n`````\nafter\n", []Kind{FenceOpen, Code, FenceClose, Prose}},
		"a fence with an info string doesn't close":          {"```\n```go\n```\n", []Kind{FenceOpen, Code, FenceClose}},
		"a fence opens on a list item's line":                {"- ```go\n  x\n  ```\nafter\n", []Kind{FenceOpen, Code, FenceClose, Prose}},
		"backticks in a backtick info string aren't a fence": {"```a```\ntext\n", []Kind{Prose, Prose}},
		"a list item's unclosed fence ends with a sibling":   {"- ```sh\n  x\n\n- [ ] next\n", []Kind{FenceOpen, Code, Code, Prose}},
		"a list item's unclosed fence ends with a paragraph": {"1. ```sh\n   x\nafter\n", []Kind{FenceOpen, Code, Prose}},
		"a list item's fence closes on an unindented marker": {"- ```sh\n  x\n```\nafter\n", []Kind{FenceOpen, Code, FenceClose, Prose}},
		"a list item's fence ends where a new one opens":     {"- ```sh\n  x\n- ```go\n  y\n", []Kind{FenceOpen, Code, FenceOpen, Code}},
	} {
		if got := kinds(c.text); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
}
