// Package slop finds AI tells in text with kit.yml's slop lexicon. Callers
// pick what's scanned (Prose for Markdown, a commit message, a reply) and
// the surface it's on; Scan reports each hit with its tier and fix.
package slop

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/md"
)

// Segment is one line of scannable text: Line is its 1-based line in the
// source, Text the line with code spans and quoted spans taken out.
type Segment struct {
	Line int
	Text string
}

// Allow is an `unslop: allow <reason>` marker, which exempts the next
// non-blank line.
type Allow struct {
	Line   int
	Reason string
}

// Hit is one tell found: where, which rule, the text it matched, and the
// rule's rewrite direction.
type Hit struct {
	Line  int
	Tier  int
	Rule  string
	Match string
	Fix   string
}

// Result is a scan's hits, in line order. Tier 3 hits are in Hits only when
// Dense: on its own a tier 3 word is noise, a cluster is the signal.
type Result struct {
	Hits  []Hit
	Dense bool
	// Score is tier 3 hits per density.per_words prose words.
	Score float64
}

// Detector is the compiled lexicon.
type Detector struct {
	rules   []rule
	words   *regexp.Regexp
	density config.SlopDensity
	block   []int
	skip    []string
}

type rule struct {
	config.SlopRule
	re *regexp.Regexp
}

// New compiles cfg's slop block, leaving out each rule whose slop-<id> slug
// disabled_rules names. A rule that doesn't compile is an error: config
// load already warned about it, so a caller fails open.
func New(cfg *config.Config) (*Detector, error) {
	s := cfg.Slop
	d := &Detector{density: s.Density, block: s.BlockTiers, skip: s.Skip}
	for _, r := range s.Rules {
		if slices.Contains(cfg.DisabledRules, "slop-"+r.ID) {
			continue
		}
		re, err := regexp.Compile(r.Match)
		if err != nil {
			return nil, fmt.Errorf("slop rule %s: %w", r.ID, err)
		}
		d.rules = append(d.rules, rule{r, re})
	}
	if len(s.Density.Words) > 0 && !slices.Contains(cfg.DisabledRules, "slop-vocabulary") {
		quoted := make([]string, len(s.Density.Words))
		for i, w := range s.Density.Words {
			quoted[i] = regexp.QuoteMeta(w)
		}
		d.words = regexp.MustCompile(`(?i)\b(?:` + strings.Join(quoted, "|") + `)\b`)
	}
	return d, nil
}

// Skips reports whether slop.skip names path.
func (d *Detector) Skips(path string) bool { return guard.GlobAny(d.skip, path) }

// Blocks reports whether h's tier is one slop.block_tiers blocks.
func (d *Detector) Blocks(h Hit) bool { return slices.Contains(d.block, h.Tier) }

// Blocking is the hits of r that block.
func (d *Detector) Blocking(r Result) []Hit {
	var out []Hit
	for _, h := range r.Hits {
		if d.Blocks(h) {
			out = append(out, h)
		}
	}
	return out
}

// Scan matches every rule that applies to surface against segs.
func (d *Detector) Scan(segs []Segment, surface string) Result {
	var hits, dense []Hit
	words := 0
	for _, seg := range segs {
		words += len(strings.Fields(seg.Text))
		for _, r := range d.rules {
			if len(r.On) > 0 && !slices.Contains(r.On, surface) {
				continue
			}
			for _, m := range r.re.FindAllString(seg.Text, -1) {
				h := Hit{seg.Line, r.Tier, r.ID, clip(m), r.Fix}
				if r.Tier == 3 {
					dense = append(dense, h)
				} else {
					hits = append(hits, h)
				}
			}
		}
		if d.words != nil {
			for _, m := range d.words.FindAllString(seg.Text, -1) {
				dense = append(dense, Hit{seg.Line, 3, "vocabulary", clip(m), ""})
			}
		}
	}
	res := Result{}
	if words > 0 && d.density.PerWords > 0 {
		res.Score = float64(len(dense)) * float64(d.density.PerWords) / float64(words)
	}
	if len(dense) > 0 && len(dense) >= d.density.Min && res.Score >= float64(d.density.Threshold) {
		res.Dense = true
		hits = append(hits, dense...)
	}
	slices.SortStableFunc(hits, func(a, b Hit) int { return a.Line - b.Line })
	res.Hits = hits
	return res
}

func clip(s string) string {
	if r := []rune(s); len(r) > 60 {
		return string(r[:57]) + "..."
	}
	return s
}

var (
	allowMarker = regexp.MustCompile(`unslop: allow\b[ \t]*(.*?)[ \t]*(?:-->|\*/)?[ \t]*$`)
	blockquote  = regexp.MustCompile(`^[ \t]*>`)
	// Straight and curly double quotes; a single quote is too often an
	// apostrophe to pair.
	quoted = regexp.MustCompile(`"[^"]*"|\x{201C}[^\x{201D}]*\x{201D}`)
)

// Prose is the scannable lines of a Markdown or plain text: prose outside
// frontmatter, fences, and blockquotes, with code spans and quoted spans
// taken out, so a tell named as an example never counts. An allow marker
// and the next non-blank line are left out; the markers come back too, for
// the caller to log.
func Prose(text string) ([]Segment, []Allow) {
	var segs []Segment
	var allows []Allow
	exempt := false
	n := 0
	for l := range md.Lines(text) {
		n++
		line := strings.TrimRight(l.Text, "\r\n")
		if l.Kind != md.Prose || strings.TrimSpace(line) == "" {
			continue
		}
		// A marker named in a code span is a mention, not a marker.
		text := stripCode(line)
		if m := allowMarker.FindStringSubmatch(text); m != nil && m[1] != "" {
			allows = append(allows, Allow{n, m[1]})
			exempt = true
			continue
		}
		if exempt {
			exempt = false
			continue
		}
		if blockquote.MatchString(line) {
			continue
		}
		segs = append(segs, Segment{n, quoted.ReplaceAllString(text, " ")})
	}
	return segs, allows
}

// stripCode replaces each code span (a backtick run, closed by a run of the
// same length) with a space. An unclosed run is left as text.
func stripCode(line string) string {
	if !strings.Contains(line, "`") {
		return line
	}
	var b strings.Builder
	for i := 0; i < len(line); {
		if line[i] != '`' {
			b.WriteByte(line[i])
			i++
			continue
		}
		run := i
		for run < len(line) && line[run] == '`' {
			run++
		}
		fence := line[i:run]
		end := closing(line, run, fence)
		if end < 0 {
			b.WriteString(fence)
			i = run
			continue
		}
		b.WriteByte(' ')
		i = end + len(fence)
	}
	return b.String()
}

// closing is where fence next appears in line from start as a whole
// backtick run, or -1.
func closing(line string, start int, fence string) int {
	for i := start; i < len(line); {
		j := strings.Index(line[i:], fence)
		if j < 0 {
			return -1
		}
		at := i + j
		end := at + len(fence)
		if end == len(line) || line[end] != '`' {
			return at
		}
		for end < len(line) && line[end] == '`' {
			end++
		}
		i = end
	}
	return -1
}
