package chatbundle

import (
	"errors"
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/md"
)

// The whole-line comments that mark what differs in chat.
const (
	markerPrefix = "<!-- chat:"
	markerSkip   = "<!-- chat: skip -->"
	markerEnd    = "<!-- chat: end -->"
	markerOnly   = "<!-- chat: only"
	markerClose  = "-->"
)

// rulesSkill is the bundled skill a rule's text lives in.
const rulesSkill = "kit-rules"

// claudeCodeKeys are the frontmatter keys chat has no use for. Chat enforces
// disable-model-invocation, and mobile has no / menu to type a skill from.
var claudeCodeKeys = []string{"allowed-tools", "argument-hint", "user-invocable", "disable-model-invocation"}

var (
	ruleLink    = regexp.MustCompile("`(rules/[a-z]+\\.md)`( sections? ([0-9]+(?:(?:,| and|, and) [0-9]+)*))?")
	skillLink   = regexp.MustCompile("`skills/([a-z0-9-]+)/(reference/[a-z0-9-]+\\.md)`")
	number      = regexp.MustCompile(`[0-9]+`)
	sectionHead = regexp.MustCompile(`^## ([0-9]+)\.`)
)

// rule is a rule file once its markers are applied: the title links name it
// by, and the sections that survived.
type rule struct {
	title    string
	sections []string
}

// parseRule reads a marked-up rule file's title and section numbers.
func parseRule(text string) (rule, error) {
	var r rule
	for line := range md.Lines(text) {
		if line.Kind != md.Prose {
			continue
		}
		if title, ok := strings.CutPrefix(line.Text, "# "); ok && r.title == "" {
			r.title = strings.TrimSpace(title)
		}
		if m := sectionHead.FindStringSubmatch(line.Text); m != nil {
			r.sections = append(r.sections, m[1])
		}
	}
	if r.title == "" {
		return r, errors.New("no # title")
	}
	return r, nil
}

// transform generates the rules skill, turns the copied files into what chat
// reads, and reports every marker or link that can't be resolved.
func (b *bundle) transform(c config.ChatBundle) []error {
	var errs []error
	rules := map[string]rule{}
	texts := []string{b.rulesHead}
	for _, name := range c.Rules {
		text, err := applyMarkers(b.rules[name])
		if err == nil {
			rules[name], err = parseRule(text)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
		texts = append(texts, text)
	}
	b.files[skillFile(rulesSkill)] = []byte(strings.Join(texts, "\n"))
	for _, name := range slices.Sorted(maps.Keys(b.files)) {
		if path.Ext(name) != ".md" {
			continue
		}
		text, fileErrs := b.chatText(name, string(b.files[name]), rules)
		errs = append(errs, fileErrs...)
		b.files[name] = []byte(text)
	}
	return errs
}

// chatText is one file as chat reads it: markers applied, links rewritten,
// and a skill's frontmatter trimmed.
func (b *bundle) chatText(name, text string, rules map[string]rule) (string, []error) {
	text, err := applyMarkers(text)
	if err != nil {
		return text, []error{fmt.Errorf("%s: %w", name, err)}
	}
	text, errs := b.rewriteLinks(name, text, rules)
	if path.Base(name) == "SKILL.md" {
		text = trimFrontmatter(text)
	}
	return text, errs
}

// applyMarkers drops what a skip marker holds and unwraps what an only
// marker holds. A marker in a fence is an example, and stays.
func applyMarkers(text string) (string, error) {
	var out strings.Builder
	open, opened, n := "", 0, 0
	lastBlank, afterMarker := true, false
	for line := range md.Lines(text) {
		n++
		marker := ""
		if line.Kind == md.Prose {
			marker = strings.TrimSpace(line.Text)
		}
		switch {
		case open == markerSkip && marker == markerEnd, open == markerOnly && marker == markerClose:
			open, afterMarker = "", true
			continue
		case strings.HasPrefix(marker, markerPrefix) && open != "":
			return "", fmt.Errorf("line %d: marker inside the one opened at line %d", n, opened)
		case marker == markerSkip, marker == markerOnly:
			open, opened = marker, n
			continue
		case strings.HasPrefix(marker, markerPrefix):
			return "", fmt.Errorf("line %d: bad marker %q", n, marker)
		case open == markerSkip:
			continue
		}
		blank := strings.TrimSpace(line.Text) == ""
		if blank && afterMarker && lastBlank {
			afterMarker = false
			continue
		}
		out.WriteString(line.Text)
		lastBlank, afterMarker = blank, false
	}
	if open != "" {
		return "", fmt.Errorf("line %d: marker never closed", opened)
	}
	return out.String(), nil
}

// rewriteLinks points each link at where chat has the text: a rule at the
// rules skill, a reference by skill name, since chat copies only a skill's
// own folder to its sandbox. What isn't bundled stays, for lint to report.
func (b *bundle) rewriteLinks(name, text string, rules map[string]rule) (string, []error) {
	var errs []error
	text = ruleLink.ReplaceAllStringFunc(text, func(link string) string {
		m := ruleLink.FindStringSubmatch(link)
		r, ok := rules[m[1]]
		if !ok {
			return link
		}
		for _, section := range number.FindAllString(m[3], -1) {
			if !slices.Contains(r.sections, section) {
				errs = append(errs, fmt.Errorf("%s: links %s section %s, which chat doesn't have", name, m[1], section))
			}
		}
		return "the " + rulesSkill + " skill, " + r.title + m[2]
	})
	text = skillLink.ReplaceAllStringFunc(text, func(link string) string {
		m := skillLink.FindStringSubmatch(link)
		if _, ok := b.files[path.Join("skills", m[1], m[2])]; !ok {
			return link
		}
		return fmt.Sprintf("`%s` in the %s skill", m[2], m[1])
	})
	return text, errs
}

// trimFrontmatter removes claudeCodeKeys. Frontmatter that doesn't parse
// stays for skillProblems to report.
func trimFrontmatter(text string) string {
	block := frontmatter(text)
	if block == "" {
		return text
	}
	var doc yaml.Node
	if yaml.Unmarshal([]byte(block), &doc) != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return text
	}
	front := doc.Content[0]
	var kept []*yaml.Node
	for pair := range slices.Chunk(front.Content, 2) {
		if !slices.Contains(claudeCodeKeys, pair[0].Value) {
			kept = append(kept, pair...)
		}
	}
	front.Content = kept
	out, err := yaml.Marshal(front)
	if err != nil {
		return text
	}
	return "---\n" + string(out) + "---\n" + text[len(block):]
}
