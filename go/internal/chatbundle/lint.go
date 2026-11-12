package chatbundle

import (
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/kitcmd"
)

// forbidden is what can't work in chat: no shell for these commands, no hook
// to run a !` block, no repo to hold these paths. A func, not a var: kit is
// also the hook binary, and no hook should compile these.
func forbidden() map[string]*regexp.Regexp {
	return map[string]*regexp.Regexp{
		"shell block":     regexp.MustCompile("(?:^|[\\s`])!`[^\\s`][^`]*`"),
		"$ARGUMENTS":      regexp.MustCompile(`\$ARGUMENTS`),
		"kit command":     regexp.MustCompile(`(?:^|[^\w-])kit (?:` + strings.Join(kitNames(), "|") + `)\b`),
		"kit.yml":         regexp.MustCompile(`kit\.yml`),
		"rule file":       regexp.MustCompile(`kit-rules/reference/[a-z]+\.md`),
		"skills/ path":    regexp.MustCompile(`skills/`),
		"AskUserQuestion": regexp.MustCompile(`AskUserQuestion`),
		"gh command":      regexp.MustCompile(`\bgh (?:api|auth|issue|pr|release|repo|run|workflow)\b`),
		"git command":     regexp.MustCompile(`\bgit (?:absorb|add|apply|bisect|blame|branch|checkout|cherry-pick|clean|clone|commit|config|diff|fetch|init|log|merge|pull|push|rebase|reflog|remote|reset|restore|revert|rev-parse|rm|show|stash|status|switch|tag|worktree)\b`),
		"CLAUDE_ env var": regexp.MustCompile(`CLAUDE_`),
	}
}

func kitNames() []string {
	names := make([]string, len(kitcmd.Commands))
	for i, c := range kitcmd.Commands {
		names[i] = regexp.QuoteMeta(c.Name)
	}
	return names
}

// skillNames matches a mention of a skill by name. A hyphenated name can
// stand bare; a one-word name, like pr or meta, reads as an ordinary word
// unless it's in code or typed with its slash.
func skillNames(skills []string) *regexp.Regexp {
	var alternatives []string
	for _, skill := range skills {
		name := regexp.QuoteMeta(skill)
		if strings.Contains(skill, "-") {
			alternatives = append(alternatives, `(?:^|[^\w-])`+name+`(?:$|[^\w-])`)
		} else {
			alternatives = append(alternatives, "(?:`/?|(?:^|\\s)/)"+name+`\b`)
		}
	}
	if len(alternatives) == 0 {
		return nil
	}
	return regexp.MustCompile(strings.Join(alternatives, "|"))
}

// lint is each line of bundled Markdown that depends on Claude Code, by
// file and line of the bundled text.
func (b *bundle) lint() []error {
	patterns := forbidden()
	if others := skillNames(b.others); others != nil {
		patterns["skill not in the bundle"] = others
	}
	labels := slices.Sorted(maps.Keys(patterns))
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(b.files)) {
		if path.Ext(name) != ".md" {
			continue
		}
		n := 0
		for line := range strings.Lines(string(b.files[name])) {
			n++
			for _, label := range labels {
				if hit := patterns[label].FindString(line); hit != "" {
					errs = append(errs, fmt.Errorf("%s:%d: %s: %s", name, n, label, strings.TrimSpace(hit)))
				}
			}
		}
	}
	return errs
}
