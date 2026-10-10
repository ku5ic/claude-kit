package main

import (
	"slices"
	"strings"
	"testing"
)

func TestParseRunChecksArgs(t *testing.T) {
	t.Parallel()
	a, err := parseRunChecksArgs([]string{"--plan", "--only", "./services/api/", "--only", "packages/a/"})
	if err != nil || !a.plan || !slices.Equal(a.only, []string{"services/api", "packages/a"}) {
		t.Errorf("--plan with --only repeated, paths cleaned: %+v, %v", a, err)
	}
	for want, args := range map[string][]string{
		`unknown argument "--plann"`:     {"--plann"},
		"--only needs at least one":      {"--only"},
		"--plan must come before --only": {"--only", ".", "--plan"},
		`unknown argument "-x"`:          {"--only", ".", "-x"},
		`unknown argument "extra"`:       {"--plan", "extra"},
	} {
		if _, err := parseRunChecksArgs(args); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", args, err, want)
		}
	}
}
