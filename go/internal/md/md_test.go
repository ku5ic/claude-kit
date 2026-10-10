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
		"frontmatter, then prose":               {"---\na: b\n---\ntext\n", []Kind{Front, Front, Front, Prose}},
		"a rule later isn't frontmatter":        {"text\n---\n", []Kind{Prose, Prose}},
		"an unclosed --- isn't frontmatter":     {"---\n```\nx\n", []Kind{Prose, FenceOpen, Code}},
		"an indented fence":                     {"  ```go\nx\n  ```\n", []Kind{FenceOpen, Code, FenceClose}},
		"a fence closes only on its own marker": {"~~~\n```\n~~~\nafter\n", []Kind{FenceOpen, Code, FenceClose, Prose}},
		"a fence never closed runs to the end":  {"```\na\nb", []Kind{FenceOpen, Code, Code}},
	} {
		if got := kinds(c.text); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
}
