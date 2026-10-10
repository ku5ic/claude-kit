package kitcmd

import (
	"strings"
	"testing"
)

// Every command is described, except one whose args leave no room.
func TestCommandsDescribed(t *testing.T) {
	for _, c := range Commands {
		if c.Desc == "" && c.Name != "git-base" {
			t.Errorf("%s has no description", c.Name)
		}
	}
}

func TestUsageLayout(t *testing.T) {
	usage := Usage()
	for _, want := range []string{
		"\n  detect-stack               compact stack report\n",
		"\n  run-checks [--plan] [--only sub...]\n                             every declared check, in every subproject;\n                             exits with the failure count.",
		"\n  git-base [--diff|--log] [base] [flags] [-- paths]\n  explain",
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
