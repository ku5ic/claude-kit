// Package layout is the row format every kit message to the user shares:
// a headline, then aligned `key  value` lines.
package layout

import (
	"fmt"
	"strings"
)

// KV is one aligned `key  value` line of a message; a key past the column
// keeps two spaces after it.
func KV(key, value string) string { return fmt.Sprintf("%-*s%s", max(6, len(key)+2), key, value) }

// Clip is s cut to its first line and 80 characters, so a heredoc or a long
// command can't wall the screen.
func Clip(s string) string {
	first, _, multi := strings.Cut(s, "\n")
	if r := []rune(first); len(r) > 80 {
		first, multi = string(r[:77]), true
	}
	if multi {
		first += "..."
	}
	return first
}
