package bashguard

import (
	"slices"
	"strings"
	"testing"
)

// names lists every command name Segments finds, in order, as "a|b" per
// pipeline.
func names(src string) []string {
	var out []string
	for _, seg := range Segments(src) {
		var calls []string
		for _, c := range seg.Calls {
			if i := lead(c.Words); i < len(c.Words) {
				calls = append(calls, baseName(c.Words[i].Value))
			}
		}
		out = append(out, strings.Join(calls, "|"))
	}
	return out
}

func TestSegmentsFindHiddenCommands(t *testing.T) {
	cases := map[string][]string{
		"echo $(true && rm -rf ~)":        {"true", "rm", "echo"},
		"echo `rm -rf ~`":                 {"rm", "echo"},
		"cat <(rm -rf ~)":                 {"rm", "cat"},
		"(rm -rf ~)":                      {"rm"},
		"{ rm -rf ~; }":                   {"rm"},
		"if true; then rm -rf ~; fi":      {"true", "rm"},
		"f() { rm -rf ~; }; f":            {"rm", "f"},
		"! rm -rf ~":                      {"rm"},
		"time rm -rf ~":                   {"rm"},
		"ls | rm -rf ~":                   {"ls|rm"},
		"env -i FOO=1 nice -n 5 rm -rf ~": {"rm"},
		"timeout -s KILL 5 rm -rf ~":      {"rm"},
		"\\rm -rf ~":                      {"rm"},
		"r''m -rf ~":                      {"rm"},
		"/bin/rm -rf ~":                   {"rm"},
		"echo hi # rm -rf ~":              {"echo"},
		"bash <<EOF\nrm -rf ~\nEOF":       {"bash", "rm"},
		"cat <<'EOF'\nrm -rf ~\nEOF":      {"cat"},
		"cat <<EOF\nrm -rf ~":             {"rm"}, // unterminated: fail closed
		"echo a\n(\nrm -rf ~":             {"echo", "rm"},
		"echo \"unterminated; rm -rf ~":   nil,
		"git commit -m \"$(cat <<'EOF'\nIt's fine\nEOF\n)\" && rm -rf ~": {"cat", "git", "rm"},
	}
	for src, want := range cases {
		if got := names(src); !slices.Equal(got, want) {
			t.Errorf("%q: got %q, want %q", src, got, want)
		}
	}
}

func TestWordValues(t *testing.T) {
	segs := Segments(`rm -rf "${HOME}" '~' $HOME/x "a b" \$y "$(kit scratch-dir)/f"`)
	// Inner substitutions come first, as they run first.
	var got []string
	for _, w := range segs[len(segs)-1].Calls[0].Words {
		got = append(got, w.Value)
	}
	want := []string{"rm", "-rf", "${HOME}", "~", "$HOME/x", "a b", "$y", "$(kit scratch-dir)/f"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRedirectTargets(t *testing.T) {
	segs := Segments(`ls > out.txt 2>&1 | tee -a log >&2`)
	var got []string
	for _, c := range segs[0].Calls {
		for _, r := range c.Redirs {
			got = append(got, r.Target.Value)
		}
	}
	if want := []string{"out.txt", "&1", "&2"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLooseWriteTarget(t *testing.T) {
	for target, want := range map[string]bool{
		"out.txt": true, "./x.md": true, "&1": false, "/dev/null": false, "-": false,
		"$(kit scratch-dir)/x": false, "scratch/x": false, "~/x": false, `"x"`: false,
	} {
		if got := looseWriteTarget(target); got != want {
			t.Errorf("looseWriteTarget(%q) = %v", target, got)
		}
	}
}
