// Package stackctx builds what inject-context (SessionStart) and
// agent-context (SubagentStart) share: the cached stack report and the
// required and suggested skills derived from it. One derivation, two
// consumers, so a subagent sees exactly the framing the main session does.
package stackctx

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/dotfiles/claude-kit/go/internal/config"
	"github.com/ku5ic/dotfiles/claude-kit/go/internal/detect"
)

// CacheFile is <cache>/stack/<name>-<sha256(root)[:8]>.<tag>.txt: the root
// is hashed in so same-named projects elsewhere on disk can't collide.
func CacheFile(paths config.Paths, cfg *config.Config, name, root string) string {
	sum := sha256.Sum256([]byte(root))
	return filepath.Join(paths.CacheDir(), "stack", name+"-"+hex.EncodeToString(sum[:])[:8]+"."+cfg.Tag+".txt")
}

// Refresh regenerates the cache when it is missing, empty, or older than
// any detection-relevant file at root or kit.yml itself (adding a stack must
// re-detect every project). Times compare in whole seconds, as stat(1) did,
// so a cache written in the same second as kit.yml still counts as fresh.
func Refresh(paths config.Paths, cfg *config.Config, root, cache string) {
	newest := int64(0)
	for _, name := range cfg.DetectFiles() {
		newest = max(newest, mtime(filepath.Join(root, name)))
	}
	newest = max(newest, mtime(paths.Base), mtime(paths.Overlay))

	if info, err := os.Stat(cache); err == nil && info.Size() > 0 && info.ModTime().Unix() >= newest {
		return
	}
	if os.MkdirAll(filepath.Dir(cache), 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(cache), ".stack-*")
	if err != nil {
		return
	}
	_, werr := tmp.WriteString(detect.Report(cfg, root))
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), cache) != nil {
		os.Remove(tmp.Name())
	}
}

func mtime(path string) int64 {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return 0
	}
	return info.ModTime().Unix()
}

var (
	skipLine   = regexp.MustCompile(`^(root|versions)[: ]`)
	extrasPart = regexp.MustCompile(`\([^)]+\)`)
)

// Signals parses a stack report into "stack" and "stack+extra" tokens in
// first-seen order: "js: yes (typescript, react) [pnpm]" yields js,
// js+typescript, js+react.
func Signals(report string) []string {
	var out []string
	for _, line := range strings.Split(report, "\n") {
		if line == "" || skipLine.MatchString(line) {
			continue
		}
		stack, _, _ := strings.Cut(line, ":")
		out = append(out, stack)
		if m := extrasPart.FindString(line); m != "" {
			for _, extra := range strings.FieldsFunc(strings.Trim(m, "()"), func(r rune) bool { return r == ',' || r == ' ' }) {
				out = append(out, stack+"+"+extra)
			}
		}
	}
	return out
}

// Required is kit.yml's global_skills, deduped in first-seen order.
func Required(cfg *config.Config) []string {
	var out []string
	for _, skill := range cfg.GlobalSkills {
		if skill != "" && !slices.Contains(out, skill) {
			out = append(out, skill)
		}
	}
	return out
}

// Suggested maps signals to the skills kit.yml gives their stack or extra,
// deduped, first-seen order, leaving out global_skills (required, not
// suggested). An extra matches by its name.
func Suggested(cfg *config.Config, signals []string) []string {
	required := Required(cfg)
	var out []string
	add := func(skills []string) {
		for _, skill := range skills {
			if skill != "" && !slices.Contains(required, skill) && !slices.Contains(out, skill) {
				out = append(out, skill)
			}
		}
	}
	for _, sig := range signals {
		stack, extra, isExtra := strings.Cut(sig, "+")
		if !isExtra {
			add(cfg.Stacks[sig].Skills)
			continue
		}
		for _, e := range cfg.Stacks[stack].Extras {
			if e.Name == extra {
				add(e.Skills)
			}
		}
	}
	return out
}

// RequiredBlock is the <required-skills> block, "" when there are none. The
// list joins on "," alone, as the bash original's "${list[*]}" under
// IFS=', ' did.
func RequiredBlock(required []string) string {
	if len(required) == 0 {
		return ""
	}
	return "\n<required-skills>\nBLOCKING REQUIREMENT: invoke the Skill tool for each of these skills NOW, before any other action: " +
		strings.Join(required, ",") + "\n</required-skills>\n"
}

// SuggestedBlock is the <suggested-skills> block, "" when there are none:
// each skill with its skill_triggers phrase when it has one.
func SuggestedBlock(cfg *config.Config, suggested []string) string {
	if len(suggested) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n<suggested-skills>\n")
	for _, skill := range suggested {
		if trigger := cfg.SkillTriggers[skill]; trigger != "" {
			b.WriteString(trigger + ": ")
		}
		b.WriteString("load " + skill + " via the Skill tool\n")
	}
	if os.Getenv("CLAUDE_GUARD_SKILLS") == "1" {
		b.WriteString("Patterns skills are also enforced automatically: the first edit to a matching file type will be blocked until the relevant skill is loaded.\n")
	}
	b.WriteString("</suggested-skills>\n")
	return b.String()
}
