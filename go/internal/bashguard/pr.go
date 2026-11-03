package bashguard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
)

var (
	// prPosts are the gh pr subcommands that post text.
	prPosts = map[string]bool{"create": true, "edit": true, "comment": true, "review": true}
	// prFlags and apiFlags are the gh flags that carry PR text, by the
	// spellings pflag accepts, to one name.
	prFlags  = map[string]string{"-b": "body", "--body": "body", "-F": "body-file", "--body-file": "body-file", "-t": "title", "--title": "title"}
	apiFlags = map[string]string{"-f": "raw-field", "--raw-field": "raw-field", "-F": "field", "--field": "field", "--input": "input"}
	// prKeys are the API fields that hold PR text.
	prKeys = []string{"body", "title"}
)

// CheckPR is guard-pr: it scans the text a gh command posts to a pull
// request (body, title, comment, review) for AI tells, as guard-commit scans
// a commit message. Text it can't read (a pipe on stdin, a file the command
// writes first) is logged, not blocked.
func CheckPR(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	cmd := h.Payload.String("tool_input.command")
	if !strings.Contains(cmd, "gh") {
		return nil
	}
	segs, _ := parseAll(cmd)
	d := dirs{home: h.Home, cwd: h.Payload.Cwd()}
	for _, seg := range segs {
		for _, call := range seg.Calls {
			i := lead(call.Words)
			if i >= len(call.Words) {
				continue
			}
			name := baseName(call.Words[i].Value)
			// In a pipeline or a subshell, cd moves nothing after it.
			if name == "cd" && len(seg.Calls) == 1 && !seg.subshell {
				d.cd(call.Words[i+1:])
			}
			if name != "gh" {
				continue
			}
			src := ghCall{call: call, before: cmd[:call.start], home: h.Home, cwd: d.cwd}
			for _, t := range src.texts(i + 1) {
				if t.unread != "" {
					h.Log(kitlog.Guards, "slop-unread", "source", t.where, "why", t.unread)
					continue
				}
				if err := scanSlop(h, t.text, "pr", t.where); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// prText is one piece of PR text a gh call posts: where it comes from, and
// its text, or why it can't be read.
type prText struct{ where, text, unread string }

// ghCall reads a gh call's text: inline, from a file under cwd, or from
// its stdin (a heredoc or a < file). before is the command ahead of the call.
type ghCall struct {
	call      Call
	before    string
	home, cwd string
}

// texts is the PR text of the gh call whose args start at word from.
func (s ghCall) texts(from int) []prText {
	var args []string
	for _, w := range s.call.Words[from:] {
		args = append(args, w.Value)
	}
	var out []prText
	switch {
	case len(args) >= 2 && args[0] == "pr" && prPosts[args[1]]:
		cmd := "gh pr " + args[1]
		ghFlags(args[2:], prFlags, func(flag, value string) {
			if flag == "body-file" {
				out = append(out, s.read(cmd+" --body-file", value))
				return
			}
			out = append(out, prText{where: cmd + " --" + flag, text: inline(value)})
		})
	case len(args) >= 1 && args[0] == "api":
		ghFlags(args[1:], apiFlags, func(flag, value string) {
			if flag == "input" {
				out = append(out, s.inputJSON(value)...)
				return
			}
			key, val, ok := strings.Cut(value, "=")
			if !ok || !slices.Contains(prKeys, key) {
				return
			}
			where := "gh api " + key + " field"
			if file, ok := strings.CutPrefix(val, "@"); ok && flag == "field" {
				out = append(out, s.read(where, file))
				return
			}
			out = append(out, prText{where: where, text: inline(val)})
		})
	}
	return out
}

// read is the text of file as gh reads it: - is stdin.
func (s ghCall) read(where, file string) prText {
	t := prText{where: where}
	switch {
	case file == "-" && len(s.call.Heredocs) > 0:
		t.text = s.call.Heredocs[0]
		return t
	case file == "-" && len(s.call.Inputs) > 0:
		file = s.call.Inputs[0]
	case file == "-":
		t.unread = "stdin isn't in the command"
		return t
	}
	switch {
	case strings.ContainsAny(file, "$`"):
		t.unread = "an expansion names the file"
	// Named before the call, the file is likely written there first: what's
	// on disk now is stale.
	case strings.Contains(s.before, filepath.Base(file)):
		t.unread = "the command writes the file first"
	default:
		data, err := os.ReadFile(fsx.Abs(s.cwd, guard.ExpandHome(s.home, file)))
		if err != nil {
			t.unread = "the file isn't readable"
		}
		t.text = string(data)
	}
	return t
}

// inputJSON is the PR text fields of the JSON request body --input reads.
func (s ghCall) inputJSON(file string) []prText {
	t := s.read("gh api --input", file)
	if t.unread != "" {
		return []prText{t}
	}
	var fields map[string]any
	if json.Unmarshal([]byte(t.text), &fields) != nil {
		return []prText{{where: t.where, unread: "the input isn't a JSON object"}}
	}
	var out []prText
	for _, key := range prKeys {
		if text, ok := fields[key].(string); ok {
			out = append(out, prText{where: "gh api --input " + key, text: text})
		}
	}
	return out
}

// inline is a flag value's text: a "$(cat <<EOF ... EOF)" value's body.
func inline(value string) string {
	if body, ok := catHeredoc(value); ok {
		return body
	}
	return value
}

// ghFlags walks gh args as pflag reads them, up to "--", calling fn with
// the name valued gives each flag it holds and the flag's value: the next
// arg, or attached (--body=x, -bx, -b=x). Other flags are skipped; one that
// takes a value leaves it as a positional arg, which no lookup matches.
func ghFlags(args []string, valued map[string]string, fn func(name, value string)) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return
		}
		flag, value, attached := strings.Cut(a, "=")
		if !strings.HasPrefix(a, "--") && strings.HasPrefix(a, "-") && len(a) > 2 {
			flag, value, attached = a[:2], strings.TrimPrefix(a[2:], "="), true
		}
		name, ok := valued[flag]
		if !ok {
			continue
		}
		if !attached && i+1 < len(args) {
			i++
			value = args[i]
		}
		fn(name, value)
	}
}
