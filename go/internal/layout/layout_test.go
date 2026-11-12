package layout

import (
	"strings"
	"testing"
)

func TestClip(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, in, want string }{
		{"short line kept", "git status", "git status"},
		{"first line of several", "cat <<EOF\nbody\nEOF", "cat <<EOF..."},
		{"long line cut to 77 characters", strings.Repeat("a", 90), strings.Repeat("a", 77) + "..."},
		{"multibyte line cut by character", strings.Repeat("é", 90), strings.Repeat("é", 77) + "..."},
		{"80 multibyte characters kept", strings.Repeat("é", 80), strings.Repeat("é", 80)},
	} {
		if got := Clip(tc.in); got != tc.want {
			t.Errorf("%s: Clip(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}
