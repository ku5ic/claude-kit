package chatbundle

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

const (
	scanSkill = "unslop"
	scanDir   = "skills/" + scanSkill

	// Linux x86_64 is what chat's sandbox runs, on desktop and on mobile.
	scanBinary = scanDir + "/scripts/kit-linux-amd64"

	lexiconPath = scanDir + "/reference/lexicon.md"
)

// addScan gives a bundled unslop skill what it needs to scan in chat: the
// script, kit.yml for the lexicon it reads, and the lexicon as a reference
// for when it can't run.
func (b *bundle) addScan(source fs.FS, cfg *config.Config) error {
	if !slices.Contains(cfg.ChatBundle.Skills, scanSkill) {
		return nil
	}
	for name, from := range map[string]string{"scripts/scan.sh": "templates/chat/scan.sh", "kit.yml": "kit.yml"} {
		data, err := fs.ReadFile(source, from)
		if err != nil {
			return err
		}
		b.files[path.Join(scanDir, name)] = data
	}
	b.files[lexiconPath] = []byte(lexicon(cfg.Slop))
	return nil
}

// addBinary is separate from addScan so --check can run with no binary built.
func (b *bundle) addBinary(root string) error {
	if _, scans := b.files[lexiconPath]; !scans {
		return nil
	}
	name := fmt.Sprintf("kit-%s-linux-amd64", b.version)
	data, err := os.ReadFile(filepath.Join(root, "bin", name))
	if err != nil {
		return fmt.Errorf("%w; go/build.sh builds it", err)
	}
	b.files[scanBinary] = data
	return nil
}

// lexicon renders the slop rules for a reader with no way to run them.
func lexicon(s config.Slop) string {
	var out strings.Builder
	out.WriteString("# Lexicon\n\nThe tells the scan looks for, to read a text against by hand when `scripts/scan.sh` can't run. Patterns are Go regular expressions; `(?i)` means case-insensitive and `\\x{2192}` names a character by code point.\n")
	for tier := 1; tier <= 3; tier++ {
		fmt.Fprintf(&out, "\n## Tier %d\n\n", tier)
		if slices.Contains(s.BlockTiers, tier) {
			out.WriteString("A match is a hit to rewrite.\n\n")
		} else {
			out.WriteString("A match counts toward density only.\n\n")
		}
		for _, r := range s.Rules {
			if r.Tier == tier {
				out.WriteString(lexiconRule(r))
			}
		}
	}
	d := s.Density
	fmt.Fprintf(&out, "\n## Density\n\nA text is dense at %d or more tier 3 matches and %d or more per %d words of prose. These words count:\n\n%s\n", d.Min, d.Threshold, d.PerWords, strings.Join(d.Words, ", "))
	return out.String()
}

func lexiconRule(r config.SlopRule) string {
	line := fmt.Sprintf("- `%s`: `%s`", r.ID, r.Match)
	if len(r.On) > 0 {
		line += " (on " + strings.Join(r.On, ", ") + ")"
	}
	if r.Fix != "" {
		line += " -> " + r.Fix
	}
	return line + "\n"
}
