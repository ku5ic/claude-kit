package kitcmd

import (
	"strings"
	"testing"
)

// A spec with room for two spaces before descColumn takes its description's
// first line; a longer one puts every line below it.
func TestUsageLayout(t *testing.T) {
	saved := Commands
	t.Cleanup(func() { Commands = saved })
	fits := strings.Repeat("x", descColumn-2-len("  edge "))
	wide := fits + "x"
	Commands = []Command{
		{Name: "short", Desc: "one line"},
		{Name: "edge", Args: fits, Desc: "at the limit"},
		{Name: "wide", Args: wide, Desc: "first\nsecond"},
		{Name: "multi", Desc: "first\nsecond"},
		{Name: "bare"},
	}
	indent := strings.Repeat(" ", descColumn)
	want := "usage: kit <command> [args]\n\n" +
		"  short" + strings.Repeat(" ", descColumn-len("  short")) + "one line\n" +
		"  edge " + fits + "  at the limit\n" +
		"  wide " + wide + "\n" + indent + "first\n" + indent + "second\n" +
		"  multi" + strings.Repeat(" ", descColumn-len("  multi")) + "first\n" + indent + "second\n" +
		"  bare\n"
	if got := Usage(); got != want {
		t.Errorf("usage:\n%s\nwant:\n%s", got, want)
	}
}

func TestReadOnly(t *testing.T) {
	if !ReadOnly("scratch-dir") || ReadOnly("run-checks") || ReadOnly("nope") {
		t.Error("scratch-dir is read-only; run-checks and unknown names aren't")
	}
}
