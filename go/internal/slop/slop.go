// Package slop finds AI tells in text with kit.yml's slop lexicon. Callers
// pick what's scanned (Prose for Markdown, a commit message, a reply) and
// the surface it's on; Scan reports each hit with its tier and fix.
package slop

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/md"
)

// Segment is one line of scannable text: Line is its 1-based line in the
// source, Text the line with code spans and quoted spans taken out. A Raw
// segment is a line of UI code as written, which only on: [ui] rules scan.
type Segment struct {
	Line int
	Text string
	Raw  bool
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
}

type rule struct {
	config.SlopRule
	re *regexp.Regexp
}

// New compiles cfg's slop block, leaving out each rule whose slop-<id> slug
// disabled_rules names, and each with an empty or uncompilable match, which
// config load already warned about: an empty one would hit everywhere.
func New(cfg *config.Config) *Detector {
	s := cfg.Slop
	d := &Detector{density: s.Density, block: s.BlockTiers}
	for _, r := range s.Rules {
		if r.Match == "" || slices.Contains(cfg.DisabledRules, "slop-"+r.ID) {
			continue
		}
		if re, err := regexp.Compile(r.Match); err == nil {
			d.rules = append(d.rules, rule{r, re})
		}
	}
	if len(s.Density.Words) > 0 && !slices.Contains(cfg.DisabledRules, "slop-vocabulary") {
		quoted := make([]string, len(s.Density.Words))
		for i, w := range s.Density.Words {
			quoted[i] = regexp.QuoteMeta(w)
		}
		d.words = regexp.MustCompile(`(?i)\b(?:` + strings.Join(quoted, "|") + `)\b`)
	}
	return d
}

// Skips reports whether cfg's slop.skip names path.
func Skips(cfg *config.Config, path string) bool { return guard.GlobAny(cfg.Slop.Skip, path) }

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

// Check scans segs on surface, hands every hit to logHit, and returns the
// result and the hits that block: what each guard does with a scan.
func (d *Detector) Check(segs []Segment, surface string, logHit func(Hit)) (Result, []Hit) {
	res := d.Scan(segs, surface)
	for _, hit := range res.Hits {
		logHit(hit)
	}
	return res, d.Blocking(res)
}

// Carried drops each hit that a line the write removes already held, by
// rule and match, one old hit for one new. A line edited in place (a typo
// fixed) keeps its old tells without blocking; a copy of an old line still
// blocks, since the old line stays.
func (d *Detector) Carried(path, before, after, surface string, hits []Hit) []Hit {
	// Swapped, Segments yields before's lines that after doesn't hold.
	removed, _, _ := Segments(path, after, before)
	old := map[[2]string]int{}
	for _, h := range d.Blocking(d.Scan(removed, surface)) {
		old[[2]string{h.Rule, strings.ToLower(h.Match)}]++
	}
	return slices.DeleteFunc(hits, func(h Hit) bool {
		k := [2]string{h.Rule, strings.ToLower(h.Match)}
		if old[k] == 0 {
			return false
		}
		old[k]--
		return true
	})
}

// LogPairs is the key/value pairs a guard logs for a hit, blocked or not.
func LogPairs(h Hit, where string) []string {
	return []string{"rule", "slop-" + h.Rule, "tier", strconv.Itoa(h.Tier), "path", where, "line", strconv.Itoa(h.Line), "match", h.Match}
}

// Message is a deny reason: each blocking hit in where with its line and
// fix, then the instruction to rewrite, not swap words.
func Message(where string, hits []Hit) string {
	var b strings.Builder
	fmt.Fprintf(&b, "unslop: %d AI tell(s) in %s:\n", len(hits), where)
	for _, h := range hits {
		fmt.Fprintf(&b, "- L%d %q -> %s\n", h.Line, h.Match, h.Fix)
	}
	b.WriteString("Rewrite each sentence around the fact it points at, or cut it; a synonym keeps the tell. Load the slop-patterns skill for how.")
	return b.String()
}

// DensityNote is the context a guard adds when r is dense.
func (d *Detector) DensityNote(where string, r Result) string {
	return fmt.Sprintf("unslop: %s reads dense with AI-era vocabulary (%.1f tier 3 hits per %d words). Not blocked; consider rewriting the passage around specifics (slop-patterns skill).", where, r.Score, d.density.PerWords)
}

// Scan matches every rule that applies to surface against segs.
func (d *Detector) Scan(segs []Segment, surface string) Result {
	var hits, dense []Hit
	words := 0
	for _, seg := range segs {
		if !seg.Raw {
			words += len(strings.Fields(seg.Text))
		}
		for _, r := range d.rules {
			if !applies(r.SlopRule, surface, seg.Raw) {
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
		if d.words != nil && !seg.Raw {
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

// applies reports whether r scans a segment on surface: a raw UI line only
// for an on: [ui] rule, any other line for a rule naming surface or none.
func applies(r config.SlopRule, surface string, raw bool) bool {
	if raw {
		return slices.Contains(r.On, "ui")
	}
	return len(r.On) == 0 || slices.Contains(r.On, surface)
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
	// quoteOpeners are the characters a quoted match starts with.
	quoteOpeners = `"` + string(rune(0x201C))
)

// Prose is the scannable lines of a Markdown or plain text: prose outside
// frontmatter, fences, and blockquotes, with code spans and quoted spans
// taken out, so a tell named as an example never counts. An allow marker
// and the next non-blank line are left out.
func Prose(text string) []Segment {
	segs, _ := finish(strings.Split(text, "\n"), proseLines(text))
	return segs
}

// proseLines is each prose line of text with its code spans taken out.
func proseLines(text string) []Segment {
	var segs []Segment
	n := 0
	for l := range md.Lines(text) {
		n++
		line := strings.TrimRight(l.Text, "\r\n")
		if l.Kind != md.Prose || strings.TrimSpace(line) == "" || blockquote.MatchString(line) {
			continue
		}
		segs = append(segs, Segment{Line: n, Text: stripCode(line)})
	}
	return segs
}

// finish takes the allow markers out of segs, sorted by line, along with
// the next non-blank line of lines after each, and the quoted spans out of
// the rest. Code spans are already gone, so a marker named in one is a
// mention, not a marker.
func finish(lines []string, segs []Segment) ([]Segment, []Allow) {
	var allows []Allow
	exempt := map[int]bool{}
	for _, s := range segs {
		if s.Raw || !strings.Contains(s.Text, "unslop: allow") {
			continue
		}
		m := allowMarker.FindStringSubmatch(s.Text)
		if m == nil || m[1] == "" {
			continue
		}
		allows = append(allows, Allow{s.Line, m[1]})
		exempt[s.Line] = true
		for n := s.Line + 1; n <= len(lines); n++ {
			if strings.TrimSpace(lines[n-1]) != "" {
				exempt[n] = true
				break
			}
		}
	}
	out := segs[:0:0]
	for _, s := range segs {
		if exempt[s.Line] {
			continue
		}
		// The regexp has no literal prefix to skip lines by; most have no quote.
		if !s.Raw && strings.ContainsAny(s.Text, quoteOpeners) {
			s.Text = quoted.ReplaceAllString(s.Text, " ")
		}
		out = append(out, s)
	}
	return out, allows
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
