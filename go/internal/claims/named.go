package claims

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

var (
	// evidenceField is a finding's Evidence line, as skills/report-format
	// writes it.
	evidenceField = regexp.MustCompile(`(?m)^\s*(?:[-*]\s*)?\*\*Evidence:\*\*\s*(.+)$`)
	lineSuffix    = regexp.MustCompile(`:\d+(-\d+)?$`)
	// cdTarget is the directory each cd in a command moves to.
	cdTarget = regexp.MustCompile(`(?:^|[;&|(]\s*)cd\s+("[^"]+"|'[^']+'|[^\s;&|)]+)`)
)

// toolCall is one tool_use's input, and the directories a relative path
// in it is read from: its entry's cwd, and each cd target in its command.
type toolCall struct {
	input string
	dirs  []string
}

// ungrounded is each Evidence field of text naming nothing the session
// touched: no backticked file:line, path, command, or value, or one that no
// tool call's input or result holds. A value counts when a result showed it,
// and a path under root when a call read it relative to its directory.
func ungrounded(root string, entries []transcript.Entry, text string) []Unsupported {
	fields := evidenceField.FindAllStringSubmatch(fence.ReplaceAllString(text, ""), -1)
	if len(fields) == 0 {
		return nil
	}
	if top := git.Toplevel(root); top != "" {
		root = top
	}
	var seen []string
	var calls []toolCall
	for pos, b := range transcript.Blocks(entries) {
		switch b.Type {
		case "tool_use":
			input := b.Input.Command + "\n" + string(b.RawInput)
			seen = append(seen, input)
			calls = append(calls, toolCall{input, callDirs(entries[pos.Entry].Cwd, b.Input.Command)})
		case "tool_result":
			seen = append(seen, b.ResultText())
		}
	}
	touched := func(core string) bool {
		return slices.ContainsFunc(seen, func(s string) bool { return strings.Contains(s, core) }) ||
			readRelative(root, core, calls)
	}
	var out []Unsupported
	for _, f := range fields {
		names := backticked.FindAllStringSubmatch(f[1], -1)
		if len(names) == 0 {
			out = append(out, Unsupported{Claim: f[1], Check: "name the file:line read or the command run, in backticks"})
			continue
		}
		for _, n := range names {
			if !touched(lineSuffix.ReplaceAllString(n[1], "")) {
				out = append(out, Unsupported{Claim: f[1], Check: "`" + n[1] + "` never ran this session: run it, or cut the finding"})
				break
			}
		}
	}
	return out
}

// callDirs is cwd and each directory a cd in command moves to, absolute.
func callDirs(cwd, command string) []string {
	var dirs []string
	if cwd != "" {
		dirs = append(dirs, cwd)
	}
	for _, m := range cdTarget.FindAllStringSubmatch(command, -1) {
		if dir := strings.Trim(m[1], `"'`); dir != "-" && !strings.HasPrefix(dir, "~") {
			dirs = append(dirs, fsx.Abs(cwd, dir))
		}
	}
	return dirs
}

// readRelative is whether a call's input names path, relative to root,
// by its path from one of the call's directories.
func readRelative(root, path string, calls []toolCall) bool {
	if root == "" || filepath.IsAbs(path) || !strings.Contains(path, "/") {
		return false
	}
	abs := filepath.Join(root, path)
	return slices.ContainsFunc(calls, func(c toolCall) bool {
		return slices.ContainsFunc(c.dirs, func(dir string) bool {
			rel, err := filepath.Rel(dir, abs)
			return err == nil && rel != path && !strings.HasPrefix(rel, "..") && strings.Contains(c.input, rel)
		})
	})
}
