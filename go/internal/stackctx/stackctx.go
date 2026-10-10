// Package stackctx builds what inject-context (SessionStart) and
// inject-subagent-context (SubagentStart) share: the cached stack report,
// and the required skills and the suggested ones: those the files and the
// declared dependencies map to. One derivation, two consumers, so a
// subagent sees exactly the framing the main session does.
package stackctx

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/detect"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// cacheFile is <cache>/stack/<name>-<sha256(root)[:8]>.<tag>.txt: the root
// is hashed in so same-named projects elsewhere on disk can't collide.
func cacheFile(paths config.Paths, cfg *config.Config, name, root string) string {
	return filepath.Join(paths.CacheDir(), cache.Stack, name+"-"+rootKey(cfg, root)+".txt")
}

// rootKey is cache.RootKey(root) and the config's tag: the suffix every
// per-project cache file shares, so a config change starts fresh files.
func rootKey(cfg *config.Config, root string) string {
	return cache.RootKey(root) + "." + cfg.Tag
}

// configTime is the newest kit.yml mtime, base or overlay: a cache older
// than it was built from a stale config.
func configTime(paths config.Paths) int64 {
	return max(mtime(paths.Base), mtime(paths.Overlay))
}

// refresh regenerates the cache when it is missing, empty, or older than
// any detection-relevant file at root, kit.yml itself (adding a stack must
// re-detect every project), or the gap-fill cache its package managers come
// from. Times compare in whole seconds, as stat(1) did, so a cache written
// in the same second as kit.yml still counts as fresh; gap-fill's answers,
// which land in the background moments after a session starts, compare in
// full.
func refresh(paths config.Paths, cfg *config.Config, root, file string) {
	newest := int64(0)
	for _, name := range cfg.DetectFiles() {
		newest = max(newest, mtime(filepath.Join(root, name)))
	}
	newest = max(newest, configTime(paths))

	answered, _ := os.Stat(gapfill.CacheFile(paths.CacheDir(), root))
	if info, err := os.Stat(file); err == nil && info.Size() > 0 && info.ModTime().Unix() >= newest &&
		(answered == nil || !answered.ModTime().After(info.ModTime())) {
		return
	}
	_ = fsx.WriteAtomic(file, []byte(detect.Report(cfg, root, paths.CacheDir())), 0o600)
}

func mtime(path string) int64 {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return 0
	}
	return info.ModTime().Unix()
}

// Context is what both context hooks derive for a project.
type Context struct {
	Report              string
	Required, Suggested []string
}

// Build refreshes root's stack cache and derives its report and skills.
func Build(paths config.Paths, cfg *config.Config, name, root, session string) Context {
	file := cacheFile(paths, cfg, name, root)
	refresh(paths, cfg, root, file)
	report, _ := os.ReadFile(file)
	c := Context{Report: string(report), Required: required(cfg)}
	c.Suggested = suggested(c.Required, FileSkills(paths, cfg, root, session), dependencySkills(cfg, root))
	return c
}

// required is kit.yml's global_skills, deduped in first-seen order.
func required(cfg *config.Config) []string {
	var out []string
	for _, skill := range cfg.GlobalSkills {
		if skill != "" && !slices.Contains(out, skill) {
			out = append(out, skill)
		}
	}
	return out
}

// suggested is the skills of lists, deduped in first-seen order, leaving
// out the required ones.
func suggested(required []string, lists ...[]string) []string {
	var out []string
	for _, skill := range slices.Concat(lists...) {
		if skill != "" && !slices.Contains(required, skill) && !slices.Contains(out, skill) {
			out = append(out, skill)
		}
	}
	return out
}

// dependencySkills are the dependency_skills skills, in rule order, for
// the dependencies root's subprojects declare: package.json's, Python
// manifests' (names normalized), and the Gemfile's.
func dependencySkills(cfg *config.Config, root string) []string {
	declared := sources.Deps{}
	for _, sub := range project.Subprojects(cfg, root) {
		dir := filepath.Join(root, sub)
		for _, deps := range []sources.Deps{sources.JSDeps(dir), sources.PythonDeps(dir), sources.RubyDeps(dir)} {
			maps.Copy(declared, deps)
		}
	}
	var out []string
	for _, rule := range cfg.DependencySkills {
		if slices.ContainsFunc(rule.Deps, func(dep string) bool { return declared[dep] || declared[sources.PyName(dep)] }) {
			out = append(out, rule.Skills...)
		}
	}
	return out
}

// FileSkills is the skill_file_map skills a file under root matches. The
// scan runs once per session and root; subagents read the cached result,
// as their SubagentStart hook has a 5s timeout. A kit.yml edit since the
// scan triggers a fresh one.
func FileSkills(paths config.Paths, cfg *config.Config, root, session string) []string {
	if root == "" {
		return nil
	}
	file := paths.SessionFile(cache.FileSkills, session, rootKey(cfg, root))
	if file != "" && mtime(file) >= configTime(paths) {
		if data, err := os.ReadFile(file); err == nil {
			return strings.Fields(string(data))
		}
	}
	skills := scanFileSkills(cfg, root)
	if file != "" {
		// Atomic: parallel SubagentStart hooks read it while one rewrites it.
		_ = fsx.WriteAtomic(file, []byte(strings.Join(skills, "\n")), 0o644)
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
			matched := FileMapSkills([]config.SkillFileRule{rule}, path)
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
	if rel, err := git.Files(root); err == nil {
		for _, file := range rel {
			files = append(files, filepath.Join(root, file))
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
func (c Context) RequiredBlock() string {
	if len(c.Required) == 0 {
		return ""
	}
	return "\n<required-skills>\nBLOCKING REQUIREMENT: invoke the Skill tool for each of these skills NOW, before any other action: " +
		strings.Join(c.Required, ",") + "\n</required-skills>\n"
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
	if SkillsEnforced() {
		b.WriteString("Patterns skills are also enforced automatically: the first edit to a matching file type will be blocked until the relevant skill is loaded.\n")
	}
	b.WriteString("</suggested-skills>\n")
	return b.String()
}

// SkillsEnforced reports whether guard-skills is on: CLAUDE_GUARD_SKILLS=1,
// an opt-in personal policy.
func SkillsEnforced() bool { return os.Getenv("CLAUDE_GUARD_SKILLS") == "1" }

// FileMapSkills is every skill a skill_file_map rule matching path gives,
// deduped: an "on: basename" rule tests the base name, "on: path" the full
// path.
func FileMapSkills(rules []config.SkillFileRule, path string) []string {
	var out []string
	for _, rule := range rules {
		target := ""
		switch rule.On {
		case "basename":
			target = filepath.Base(path)
		case "path":
			target = path
		default:
			continue
		}
		// An extension's case varies (Main.GO on a case-insensitive disk); a name's doesn't.
		ext := filepath.Ext(target)
		if !guard.GlobAny(rule.Globs, target) && !guard.GlobAny(rule.Globs, strings.TrimSuffix(target, ext)+strings.ToLower(ext)) {
			continue
		}
		for _, skill := range rule.Skills {
			if skill != "" && !slices.Contains(out, skill) {
				out = append(out, skill)
			}
		}
	}
	return out
}
