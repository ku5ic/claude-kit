package kitcmd

import (
	"strings"
	"testing"
)

// Every command is described; one whose args leave no room wraps.
func TestCommandsDescribed(t *testing.T) {
	for _, c := range Commands {
		if c.Desc == "" {
			t.Errorf("%s has no description", c.Name)
		}
	}
}

func TestUsageLayout(t *testing.T) {
	usage := Usage()
	for _, want := range []string{
		"\n  detect-stack               compact stack report\n",
		"\n  run-checks [--plan] [--all] [--only sub...]\n                             every declared check, in every subproject,\n                             on what changed since the git base (--all:\n                             the whole tree); exits with the failure count.",
		"\n  git-base [--diff|--log] [base] [flags] [-- paths]\n                             the base ref of this checkout, or the diff\n                             or log against it\n  explain",
	} {
		if !strings.Contains(usage, want) {
			t.Errorf("usage lacks %q:\n%s", want, usage)
		}
	}
}

func TestReadOnly(t *testing.T) {
	if !ReadOnly("scratch-dir") || ReadOnly("run-checks") || ReadOnly("nope") {
		t.Error("scratch-dir is read-only; run-checks and unknown names aren't")
	}
}
