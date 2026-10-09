// Package stackctx builds what inject-context (SessionStart) and
// agent-context (SubagentStart) share: the cached stack report and the
// required and suggested skills derived from it. One derivation, two
// consumers, so a subagent sees exactly the framing the main session does.
package stackctx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/detect"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// CacheFile is <cache>/stack/<name>-<sha256(root)[:8]>.<tag>.txt: the root
// is hashed in so same-named projects elsewhere on disk can't collide.
func CacheFile(paths config.Paths, cfg *config.Config, name, root string) string {
	return filepath.Join(paths.CacheDir(), "stack", name+"-"+rootKey(cfg, root)+".txt")
}

// rootKey is <sha256(root)[:8]>.<tag>, the suffix every per-project cache
// file shares.
func rootKey(cfg *config.Config, root string) string {
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:])[:8] + "." + cfg.Tag
}

// configTime is the newest kit.yml mtime, base or overlay: a cache older
// than it was built from a stale config.
func configTime(paths config.Paths) int64 {
	return max(mtime(paths.Base), mtime(paths.Overlay))
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
	newest = max(newest, configTime(paths))

	if info, err := os.Stat(cache); err == nil && info.Size() > 0 && info.ModTime().Unix() >= newest {
		return
	}
	if os.MkdirAll(filepath.Dir(cache), 0o755) != nil {
		return
	}
	_ = project.WriteAtomic(cache, []byte(detect.Report(cfg, root)), 0o600)
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
	for line := range strings.SplitSeq(report, "\n") {
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

// Context is what both context hooks derive for a project.
type Context struct {
	Report              string
	Required, Suggested []string
}

// Build refreshes root's stack cache and derives its report and skills;
// Suggested stays empty when there's no report.
func Build(paths config.Paths, cfg *config.Config, name, root, session string) Context {
	cache := CacheFile(paths, cfg, name, root)
	Refresh(paths, cfg, root, cache)
	report, _ := os.ReadFile(cache)
	c := Context{Report: string(report), Required: Required(cfg)}
	if c.Report != "" {
		c.Suggested = Suggested(cfg, Signals(c.Report), FileSkills(paths, cfg, root, session))
	}
	return c
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
// suggested). An extra matches by its name. fileSkills, from FileSkills,
// follow the stack skills.
func Suggested(cfg *config.Config, signals, fileSkills []string) []string {
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
	add(fileSkills)
	return out
}

// FileSkills is, with CLAUDE_GUARD_SKILLS=1, the skill_file_map skills a
// file under root matches, since guard-skills blocks the first edit of such
// a file without them. The scan runs once per session and root; subagents
// read the cached result, as their SubagentStart hook has a 5s timeout. A
// kit.yml edit since the scan triggers a fresh one.
func FileSkills(paths config.Paths, cfg *config.Config, root, session string) []string {
	if !guard.SkillsEnforced() || root == "" {
		return nil
	}
	cache := paths.SessionFile(config.FileSkills, session, rootKey(cfg, root))
	if cache != "" && mtime(cache) >= configTime(paths) {
		if data, err := os.ReadFile(cache); err == nil {
			return strings.Fields(string(data))
		}
	}
	skills := scanFileSkills(cfg, root)
	if cache != "" && os.MkdirAll(filepath.Dir(cache), 0o755) == nil {
		_ = os.WriteFile(cache, []byte(strings.Join(skills, "\n")), 0o644)
	}
	return skills
}

// scanFileSkills matches skill_file_map against every file git lists under
// root, tracked or untracked and not ignored, or a capped walk outside a
// git work tree. A rule leaves the scan once it matches; the scan ends early
// only when every rule has.
func scanFileSkills(cfg *config.Config, root string) []string {
	rules := slices.Clone(cfg.SkillFileMap)
	var skills []string
	for _, path := range listFiles(cfg, root) {
		rules = slices.DeleteFunc(rules, func(rule config.SkillFileRule) bool {
			matched := guard.FileMapSkills([]config.SkillFileRule{rule}, path)
			skills = append(skills, matched...)
			return len(matched) > 0
		})
		if len(rules) == 0 {
			break
		}
	}
	return skills
}

// walkCap bounds the non-git walk: a project without git is rarely large,
// and a home-sized tree must not stall a 5s hook.
const walkCap = 20000

// listFiles is every file under root as an absolute path: git's list when
// root is a work tree, else a walk that skips hidden and skip_dirs dirs,
// stopping at walkCap files.
func listFiles(cfg *config.Config, root string) []string {
	var files []string
	if out, err := git.Output(root, "ls-files", "--cached", "--others", "--exclude-standard", "-z"); err == nil {
		for file := range strings.SplitSeq(out, "\x00") {
			if file != "" {
				files = append(files, filepath.Join(root, file))
			}
		}
		return files
	}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return nil
		case d.IsDir() && path != root && (strings.HasPrefix(d.Name(), ".") || slices.Contains(cfg.SkipDirs, d.Name())):
			return filepath.SkipDir
		case d.IsDir():
			return nil
		case len(files) >= walkCap:
			return filepath.SkipAll
		}
		files = append(files, path)
		return nil
	})
	return files
}

// RequiredBlock is the <required-skills> block, "" when there are none.
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
			fmt.Fprintf(&b, "%s: ", trigger)
		}
		fmt.Fprintf(&b, "load %s via the Skill tool\n", skill)
	}
	if guard.SkillsEnforced() {
		b.WriteString("Patterns skills are also enforced automatically: the first edit to a matching file type will be blocked until the relevant skill is loaded.\n")
	}
	b.WriteString("</suggested-skills>\n")
	return b.String()
}
