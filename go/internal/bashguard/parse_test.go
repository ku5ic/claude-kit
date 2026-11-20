package bashguard

import (
	"fmt"
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
		// A segment of redirects alone ({ cat; } < f) names no command.
		if len(calls) > 0 {
			out = append(out, strings.Join(calls, "|"))
		}
	}
	return out
}

// hiddenCommands maps a command line to the names Segments finds in it.
var hiddenCommands = map[string][]string{
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
	"true | >f; rm -rf ~":            {"true", "rm"},
	"true | (echo) > $(rm -rf ~)":    {"echo", "rm", "true"},
	"curl x | time sh":               {"curl|sh"},
	"time -- rm -rf ~":               {"rm"},
	"time":                           nil,
	"sudo bash <<EOF\nrm -rf ~\nEOF": {"bash", "rm"},
	"ash <<EOF\nrm -rf ~\nEOF":       {"ash", "rm"},
}

func TestSegmentsFindHiddenCommands(t *testing.T) {
	for src, want := range hiddenCommands {
		if got := names(src); !slices.Equal(got, want) {
			t.Errorf("%q: got %q, want %q", src, got, want)
		}
	}
}

// FuzzSegments checks that no input panics the parser: a panic fails every
// Bash guard open.
func FuzzSegments(f *testing.F) {
	for src := range hiddenCommands {
		f.Add(src)
	}
	for _, src := range []string{"0|>0", "echo x | >out.txt", "true | { echo; } > $(rm -rf ~)", "time; rm -rf ~", "echo | time; rm -rf ~"} {
		f.Add(src)
	}
	f.Fuzz(func(t *testing.T, src string) {
		Segments(src)
	})
}

// A reparse after an error is a sibling of the parse it follows, so any
// number of bad lines leaves the next one read; only nesting is capped.
func TestParseErrorsDontHideLaterLines(t *testing.T) {
	src := strings.Repeat("echo $((ls) )\n", 12) + "rm -rf ~"
	if got := names(src); !slices.Equal(got, []string{"rm"}) {
		t.Errorf("after 12 bad lines: got %q, want [rm]", got)
	}
	if got := names(strings.Repeat("echo $((ls) ); ", 12) + "rm -rf ~"); !slices.Equal(got, []string{"rm"}) {
		t.Errorf("after 12 bad statements on one line: got %q, want [rm]", got)
	}
	var nested strings.Builder
	for i := range 9 {
		fmt.Fprintf(&nested, "bash <<E%d\n", i)
	}
	nested.WriteString("rm -rf ~\n")
	for i := 8; i >= 0; i-- {
		fmt.Fprintf(&nested, "E%d\n", i)
	}
	if _, unparsed := parseAll(nested.String()); !unparsed {
		t.Error("a heredoc nested 9 deep isn't unparsed")
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
