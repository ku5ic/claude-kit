package bashguard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/slop"
)

var (
	// The gh flags that carry PR text, by the spellings pflag accepts, to
	// one name. Per subcommand: pr review's -c is a bool, merge's -t the
	// commit subject.
	textFlags    = map[string]string{"-b": "body", "--body": "body", "-F": "body-file", "--body-file": "body-file", "-t": "title", "--title": "title"}
	mergeFlags   = map[string]string{"-b": "body", "--body": "body", "-F": "body-file", "--body-file": "body-file", "-t": "subject", "--subject": "subject"}
	commentFlags = map[string]string{"-c": "comment", "--comment": "comment"}
	apiFlags     = map[string]string{"-f": "raw-field", "--raw-field": "raw-field", "-F": "field", "--field": "field", "--input": "input"}
	// ghPosts are the gh pr and gh issue subcommands that post text, each
	// with the flags that carry it.
	ghPosts = map[string]map[string]map[string]string{
		"pr": {"create": textFlags, "edit": textFlags, "comment": textFlags, "review": textFlags,
			"merge": mergeFlags, "close": commentFlags, "reopen": commentFlags},
		"issue": {"create": textFlags, "edit": textFlags, "comment": textFlags, "close": commentFlags, "reopen": commentFlags},
	}
	// prKeys are the API fields that hold PR text.
	prKeys = []string{"body", "title"}
)

// CheckPR is guard-pr: it scans the text a gh command posts to a pull
// request or an issue (body, title, comment, review) for AI tells, as guard-commit scans
// a commit message. Text it can't read (a pipe on stdin, a file the command
// writes first, an expansion) is logged, not blocked.
func CheckPR(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	cmd := h.Payload.String("tool_input.command")
	if !strings.Contains(cmd, "gh") {
		return nil
	}
	segs, _ := parseAll(cmd)
	var det *slop.Detector
	for w := range walk(segs, h.Home, h.Payload.Cwd()) {
		if w.name != "gh" {
			continue
		}
		// call.start is an offset into seg.src, which for a heredoc fed to a
		// shell is that body, not cmd.
		src := ghCall{call: w.call, before: w.seg.src[:min(w.call.start, len(w.seg.src))], home: h.Home, cwd: w.cwd}
		for _, t := range src.texts(w.i + 1) {
			if t.unread != "" {
				h.Log(kitlog.Guards, "slop-unread", "source", t.where, "why", t.unread)
				continue
			}
			if det == nil {
				det = detector(h) // once per command, not per text
			}
			if err := scanSlop(h, det, t.text, "pr", t.where); err != nil {
				return err
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
	case len(args) >= 2 && ghPosts[args[0]][args[1]] != nil:
		cmd := "gh " + args[0] + " " + args[1]
		ghFlags(args[2:], ghPosts[args[0]][args[1]], func(flag, value string) {
			if flag == "body-file" {
				out = append(out, s.read(cmd+" --body-file", value))
				return
			}
			out = append(out, inline(cmd+" --"+flag, value))
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
			out = append(out, inline(where, val))
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

// inline is a flag value's text: a "$(cat <<EOF ... EOF)" value's body, or
// unread when any other expansion builds it ($(cat f), $BODY, backticks).
func inline(where, value string) prText {
	if body, ok := catHeredoc(value); ok {
		return prText{where: where, text: body}
	}
	if expansion.MatchString(value) {
		return prText{where: where, unread: "an expansion builds the text"}
	}
	return prText{where: where, text: value}
}

// expansion is a $ or backtick the shell expands; $5 isn't one.
var expansion = regexp.MustCompile("\\$[({A-Za-z_]|`")

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
