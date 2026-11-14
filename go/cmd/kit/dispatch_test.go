package main

import (
	"slices"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitcmd"
)

// unlisted are the commands kitcmd.Commands leaves out of the usage.
var unlisted = []string{"help", "-h", "--help", "--version"}

// Every command kitcmd lists has a handler, and every handler is listed or
// an alias: usage and completion read kitcmd, run reads commands.
func TestEveryListedCommandIsDispatched(t *testing.T) {
	t.Parallel()
	var listed []string
	for _, c := range kitcmd.Commands {
		listed = append(listed, c.Name)
		if _, ok := commands[c.Name]; !ok {
			t.Errorf("kit %s is listed in kitcmd.Commands, but commands has no handler for it", c.Name)
		}
	}
	for name := range commands {
		if !slices.Contains(listed, name) && !slices.Contains(unlisted, name) {
			t.Errorf("kit %s has a handler, but kitcmd.Commands doesn't list it", name)
		}
	}
}
