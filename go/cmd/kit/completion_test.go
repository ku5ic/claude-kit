package main

import "testing"

func TestZshQuote(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"explain:why a guard decides": `'explain:why a guard decides'`,
		"x:a subagent's context":      `'x:a subagent'\''s context'`,
		"x:path: a file":              `'x:path\: a file'`,
		"git-base":                    `'git-base'`,
	} {
		if got := zshQuote(in); got != want {
			t.Errorf("zshQuote(%q) = %s, want %s", in, got, want)
		}
	}
}
