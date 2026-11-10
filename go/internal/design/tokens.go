package design

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/git"
)

var (
	// uiExts are the files a token is used in and defined in.
	uiExts = []string{".css", ".scss", ".sass", ".less", ".tsx", ".jsx", ".ts", ".js", ".vue", ".svelte", ".astro", ".html", ".htm", ".mdx"}
	// varUse is a var() with no fallback.
	varUse = regexp.MustCompile(`var\(\s*(--[\w-]+)\s*\)`)
	// propDef is a custom property set: in CSS, an inline style object, or setProperty.
	propDef = regexp.MustCompile(`(--[\w-]+)['"]?\s*:|setProperty\(\s*['"](--[\w-]+)`)
	// shadedColor is a Tailwind color utility with a palette shade.
	// Longer prefixes come first: RE2 takes the first alternative that matches.
	shadedColor    = regexp.MustCompile(`(?:^|[\s"'\x60:{(!])(?:text-shadow|inset-shadow|inset-ring|drop-shadow|ring-offset|border(?:-[xytrblse])?|bg|text|ring|outline|fill|stroke|from|via|to|decoration|accent|caret|divide|placeholder|shadow)-([a-z]+(?:-[a-z]+)*)-(50|[1-9]00|950)(?:/\d+)?\b`)
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
// file sets, and, in a Tailwind v4 project configured in CSS, a shaded
// color utility (bg-brand-500) whose color is neither a default palette
// color nor a --color-* the project sets. nil when path isn't a UI file,
// isn't in a git repository, or the project is too big to read.
func UndefinedTokens(path, before, after string) []string {
	if !slices.Contains(uiExts, strings.ToLower(filepath.Ext(path))) {
		return nil
	}
	root := git.Toplevel(filepath.Dir(path))
	if root == "" {
		return nil
	}
	files, err := git.Files(root)
	if err != nil || len(files) > maxFiles {
		return nil
	}
	defined := map[string]bool{}
	define := func(text string) {
		for _, m := range propDef.FindAllStringSubmatch(text, -1) {
			defined[m[1]+m[2]] = true
		}
	}
	define(after)
	tailwind, jsConfig := false, false
	for _, f := range files {
		base := filepath.Base(f)
		if strings.HasPrefix(base, "tailwind.config.") {
			jsConfig = true
		}
		if !slices.Contains(uiExts, strings.ToLower(filepath.Ext(f))) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			continue
		}
		define(string(data))
		if !tailwind && tailwindImport.Match(data) {
			tailwind = true
		}
	}
	var out []string
	for _, v := range added(varUse, before, after) {
		if !defined[v] && (!tailwind || !tailwindVar(v)) {
			out = append(out, "var("+v+")")
		}
	}
	if tailwind && !jsConfig {
		old := map[string]bool{}
		for _, m := range shadedColor.FindAllStringSubmatch(before, -1) {
			old[m[1]+"-"+m[2]] = true
		}
		for _, m := range shadedColor.FindAllStringSubmatch(after, -1) {
			token := "--color-" + m[1] + "-" + m[2]
			if old[m[1]+"-"+m[2]] || slices.Contains(palette, m[1]) || defined[token] || slices.Contains(out, token) {
				continue
			}
			out = append(out, token)
		}
	}
	return out
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
