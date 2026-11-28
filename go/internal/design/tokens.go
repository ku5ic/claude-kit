package design

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
)

var (
	// uiExts are the files a token is used in and defined in.
	uiExts = []string{".css", ".scss", ".sass", ".less", ".tsx", ".jsx", ".ts", ".js", ".vue", ".svelte", ".astro", ".html", ".htm", ".mdx"}
	// varUse is a var() with no fallback.
	varUse = regexp.MustCompile(`var\(\s*(--[\w-]+)\s*\)`)
	// propDef is a custom property set: in CSS, an inline style object, or setProperty.
	propDef = regexp.MustCompile(`(--[\w-]+)['"]?\s*:|setProperty\(\s*['"](--[\w-]+)`)
	// shadedColor is a Tailwind color utility with a palette shade; its
	// group is the color and shade, brand-500.
	// Longer prefixes come first: RE2 takes the first alternative that matches.
	shadedColor    = regexp.MustCompile(`(?:^|[\s"'\x60:{(!])(?:text-shadow|inset-shadow|inset-ring|drop-shadow|ring-offset|border(?:-[xytrblse])?|bg|text|ring|outline|fill|stroke|from|via|to|decoration|accent|caret|divide|placeholder|shadow)-([a-z]+(?:-[a-z]+)*-(?:50|[1-9]00|950))(?:/\d+)?\b`)
	tailwindImport = regexp.MustCompile(`@import\s+['"]tailwindcss['"]`)
	// palette is Tailwind v4's default colors (tailwindcss.com/docs/colors).
	palette = []string{"red", "orange", "amber", "yellow", "lime", "green", "emerald", "teal", "cyan", "sky", "blue", "indigo",
		"violet", "purple", "fuchsia", "pink", "rose", "slate", "gray", "zinc", "neutral", "stone", "mauve", "olive", "mist", "taupe"}
	// tailwindVars are the theme variable namespaces Tailwind v4 defines
	// itself, so a project using Tailwind never declares them.
	tailwindVars = []string{"--spacing", "--font-", "--text-", "--radius-", "--shadow-", "--inset-shadow-", "--drop-shadow-",
		"--text-shadow-", "--breakpoint-", "--container-", "--tracking-", "--leading-", "--ease-", "--animate-", "--blur-",
		"--perspective-", "--aspect-", "--default-", "--tw-", "--color-black", "--color-white"}
)

// maxFiles caps how many project files a check reads.
// shortcut: a project past it isn't checked; an index of definitions, kept
// fresh per edit, would lift the cap.
const maxFiles = 5000

// UndefinedTokens is each design token an edit of path adds that the
// project doesn't define: a var(--x) with no fallback whose --x no project
// file sets, in a namespace (--x-*) the project sets others in, and, in a
// Tailwind v4 project configured in CSS, a shaded color utility
// (bg-brand-500) whose color is neither a default palette color nor a
// --color-* the project sets. A var in a namespace the project never sets
// is a dependency's, which git doesn't list. nil when path isn't a UI
// file, isn't in a git repository, or the project is too big to read.
// cacheDir holds what each project file defines, "" for none.
func UndefinedTokens(cacheDir, path, before, after string) []string {
	if !slices.Contains(uiExts, strings.ToLower(filepath.Ext(path))) {
		return nil
	}
	vars, colors := added(varUse, before, after), addedColors(before, after)
	if len(vars) == 0 && len(colors) == 0 {
		return nil
	}
	root := git.Toplevel(filepath.Dir(path))
	if root == "" {
		return nil
	}
	project, ok := readProject(root, cacheDir)
	if !ok {
		return nil
	}
	return project.undefined(vars, colors, after)
}

// addedColors is the shaded color utilities after adds to before that
// aren't default palette colors.
func addedColors(before, after string) []string {
	var colors []string
	for _, c := range added(shadedColor, before, after) {
		if !slices.Contains(palette, c[:strings.LastIndex(c, "-")]) {
			colors = append(colors, c)
		}
	}
	return colors
}

// undefined is each of vars and colors that neither p nor after defines,
// a var only in a namespace p owns.
func (p projectTokens) undefined(vars, colors []string, after string) []string {
	defined := maps.Clone(p.defined)
	define(defined, after)
	tailwind, jsConfig := p.tailwind, p.jsConfig
	owned := map[string]bool{}
	for v := range defined {
		owned[namespace(v)] = true
	}
	var out []string
	for _, v := range vars {
		if !defined[v] && owned[namespace(v)] && (!tailwind || !tailwindVar(v)) {
			out = append(out, "var("+v+")")
		}
	}
	if tailwind && !jsConfig {
		for _, c := range colors {
			if token := "--color-" + c; !defined[token] {
				out = append(out, token)
			}
		}
	}
	return out
}

// projectTokens is what a project's UI files define, and how it sets up
// Tailwind.
type projectTokens struct {
	defined            map[string]bool
	tailwind, jsConfig bool
}

// projects memoizes readProject per root, a failure too: bash-edits checks
// every file a command changed in one process, and each would read the
// project again.
var projects sync.Map

// read is one readProject result.
type read struct {
	tokens projectTokens
	ok     bool
}

// readProject is scanProject, once per process.
func readProject(root, cacheDir string) (projectTokens, bool) {
	if r, ok := projects.Load(root); ok {
		return r.(read).tokens, r.(read).ok
	}
	p, ok := scanProject(root, cacheDir)
	projects.Store(root, read{p, ok})
	return p, ok
}

// fileTokens is what one UI file defines, and the size and mtime it had.
type fileTokens struct {
	Size, MTime int64
	Defined     []string
	Tailwind    bool
}

// scanProject reads every UI file git lists under root, but one whose size
// and mtime match the index under cacheDir, which answers for it; false
// when root isn't listable or has more than maxFiles files.
func scanProject(root, cacheDir string) (projectTokens, bool) {
	files, err := git.Files(root)
	if err != nil || len(files) > maxFiles {
		return projectTokens{}, false
	}
	index := ""
	if cacheDir != "" {
		index = indexFile(cacheDir, root)
	}
	old := loadIndex(index)
	now := make(map[string]fileTokens, len(old))
	changed := false
	p := projectTokens{defined: map[string]bool{}}
	for _, f := range files {
		if strings.HasPrefix(filepath.Base(f), "tailwind.config.") {
			p.jsConfig = true
		}
		if !slices.Contains(uiExts, strings.ToLower(filepath.Ext(f))) {
			continue
		}
		cached, hit := old[f]
		t, fresh, ok := scanFile(filepath.Join(root, f), cached, hit)
		if !ok {
			continue
		}
		changed = changed || fresh
		now[f] = t
		for _, v := range t.Defined {
			p.defined[v] = true
		}
		p.tailwind = p.tailwind || t.Tailwind
	}
	if index != "" && (changed || len(now) != len(old)) {
		var buf bytes.Buffer
		if gob.NewEncoder(&buf).Encode(now) == nil {
			_ = fsx.WriteAtomic(index, buf.Bytes(), 0o600)
		}
	}
	return p, true
}

// scanFile is what the UI file at path defines: cached when hit and its
// size and mtime match cached's, else read again and fresh. ok is false
// when path can't be read.
func scanFile(path string, cached fileTokens, hit bool) (t fileTokens, fresh, ok bool) {
	info, err := os.Stat(path)
	if err != nil {
		return fileTokens{}, false, false
	}
	if hit && cached.Size == info.Size() && cached.MTime == info.ModTime().UnixNano() {
		return cached, false, true
	}
	// The stat before the read: a write between them shows as a newer
	// mtime next time, never as a stale hit.
	data, err := os.ReadFile(path)
	if err != nil {
		return fileTokens{}, false, false
	}
	defined := map[string]bool{}
	define(defined, string(data))
	return fileTokens{Size: info.Size(), MTime: info.ModTime().UnixNano(), Defined: slices.Sorted(maps.Keys(defined)), Tailwind: tailwindImport.Match(data)}, true, true
}

// indexVersion names the index's shape and how define reads a file: a change
// to either bumps it, so an index an older kit wrote is never read.
const indexVersion = 1

// indexFile is root's index under cacheDir, for this indexVersion.
func indexFile(cacheDir, root string) string {
	return filepath.Join(cacheDir, cache.Tokens, fmt.Sprintf("%s-v%d.gob", cache.RootKey(root), indexVersion))
}

// loadIndex is the index at file, by path from the root; empty on any
// miss, which costs only a full read.
func loadIndex(file string) map[string]fileTokens {
	index := map[string]fileTokens{}
	if data, err := os.ReadFile(file); err == nil && gob.NewDecoder(bytes.NewReader(data)).Decode(&index) != nil {
		return map[string]fileTokens{}
	}
	return index
}

// define adds each custom property text sets to defined.
func define(defined map[string]bool, text string) {
	for _, m := range propDef.FindAllStringSubmatch(text, -1) {
		defined[m[1]+m[2]] = true
	}
}

// namespace is the first word of a custom property, as accent is of --accent-9.
func namespace(v string) string {
	name, _, _ := strings.Cut(strings.TrimPrefix(v, "--"), "-")
	return name
}

// added is each first group re matches in after and not in before, once.
func added(re *regexp.Regexp, before, after string) []string {
	old := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(before, -1) {
		old[m[1]] = true
	}
	var out []string
	for _, m := range re.FindAllStringSubmatch(after, -1) {
		if !old[m[1]] && !slices.Contains(out, m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

// tailwindVar is true for a variable in a namespace Tailwind defines, or a
// default palette color.
func tailwindVar(v string) bool {
	if slices.ContainsFunc(tailwindVars, func(p string) bool { return strings.HasPrefix(v, p) }) {
		return true
	}
	color, ok := strings.CutPrefix(v, "--color-")
	if !ok {
		return false
	}
	name, _, _ := strings.Cut(color, "-")
	return slices.Contains(palette, name)
}
