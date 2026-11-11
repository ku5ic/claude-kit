// Package layout is the row format every kit message to the user shares:
// a headline, then aligned `key  value` lines.
package layout

import (
	"fmt"
	"strings"
)

// KV is one aligned `key  value` line of a message.
func KV(key, value string) string { return fmt.Sprintf("%-6s%s", key, value) }

// Clip is s cut to its first line and 80 characters, so a heredoc or a long
// command can't wall the screen.
func Clip(s string) string {
	first, _, multi := strings.Cut(s, "\n")
	if len(first) > 80 {
		first, multi = first[:77], true
	}
	if multi {
		first += "..."
	}
	return first
}
