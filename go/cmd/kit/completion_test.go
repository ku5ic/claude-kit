package main

import (
	"slices"
	"testing"
)

func TestParseUsage(t *testing.T) {
	tests := []struct {
		name, text string
		want       []command
	}{
		{
			name: "an arg spec crossing the description column stays out of the description",
			text: "  git-base [--diff|--log] [ref] [flags] [-- paths]\n",
			want: []command{{name: "git-base"}},
		},
		{
			name: "continuation lines join the description",
			text: "  run-checks [--only sub...]  every declared check;\n                             exits with the failure count\n",
			want: []command{{name: "run-checks", desc: "every declared check; exits with the failure count"}},
		},
		{
			name: "a description on continuation lines only",
			text: "  blast-radius <file> [symbol]\n                             the files that import <file>\n",
			want: []command{{name: "blast-radius", desc: "the files that import <file>"}},
		},
		{
			name: "blank-ish lines and a continuation before any command are skipped",
			text: "usage: kit\n  \n     stray note\n  version                    the plugin version\n",
			want: []command{{name: "version", desc: "the plugin version"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseUsage(tt.text); !slices.Equal(got, tt.want) {
				t.Errorf("parseUsage = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// Every command in the real usage text must parse with a non-empty
// description except those whose arg spec leaves no room for one.
func TestUsageCommandsDescribed(t *testing.T) {
	for _, c := range usageCommands() {
		if c.desc == "" && c.name != "git-base" {
			t.Errorf("%s has no description", c.name)
		}
	}
}
