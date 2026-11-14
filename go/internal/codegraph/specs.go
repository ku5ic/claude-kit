package codegraph

import (
	"regexp"
	"strings"
	"unicode"
)

// An import's text is a bare spec ("react", "./x") or a whole statement
// ("use crate::a::b;", "#include <x.h>", "import static a.B.C;").
var (
	keyword = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?` +
		`(#\s*include|import|using|use|alias|require_relative|require|from|mod|extern\s+crate)(\s+|[("'<])`)
	staticWord  = regexp.MustCompile(`^static\s+`)
	specEnd     = regexp.MustCompile(`\s+as\s+|[;,\s()]`)
	csharpAlias = regexp.MustCompile(`^\w+\s*=`)
	pyRelative  = regexp.MustCompile(`^(\.+)([^./].*)?$`)
	sourceFile  = regexp.MustCompile(`\.(h|hh|hpp|hxx|inc|c|cc|cpp|js|jsx|mjs|cjs|ts|tsx|json|css|scss|sass|less|vue|svelte` +
		`|rb|py|go|rs|php|dart|lua|graphql|gql|html|svg|png|md|ya?ml|wasm)$`)
)

var (
	domainRoots   = set("com", "org", "net", "io", "dev", "edu", "gov", "co")
	platformRoots = set("java", "javax", "jakarta", "kotlin", "kotlinx", "android", "androidx", "scala", "System", "Microsoft")
	// Languages whose specs are always dotted names, so a.b.json is a module, not a file.
	dottedLanguages = set(".py", ".java", ".kt", ".kts", ".cs", ".scala", ".groovy", ".lua", ".ex", ".exs")
	// NodeNext imports name the emitted file: ./a.js is a.ts.
	jsEmitted  = set(".js", ".jsx", ".mjs", ".cjs")
	ecosystems = map[string]string{
		".js": "npm", ".jsx": "npm", ".mjs": "npm", ".cjs": "npm", ".ts": "npm", ".tsx": "npm", ".mts": "npm", ".cts": "npm",
		".vue": "npm", ".svelte": "npm", ".py": "python", ".rs": "rust", ".rb": "ruby", ".go": "go",
	}
)

func set(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, item := range items {
		m[item] = true
	}
	return m
}

// spec is one import target, path-like, and how it resolves.
type spec struct {
	path string
	kind specKind
}

type specKind int

const (
	plain    specKind = iota // a bare name: a package, or a path under a source root
	dotted                   // a.b.c, a module path in a dotted language
	rooted                   // a path from the repo root
	relative                 // a path from the importing file
)

// rawSpecs is the specs in an import's text, each marked when the
// language makes it file-relative (require_relative, #include "x").
func rawSpecs(text string) []spec {
	text = strings.TrimSpace(text)
	kw := ""
	if m := keyword.FindStringSubmatchIndex(text); m != nil {
		kw = text[m[2]:m[3]]
		end := m[1]
		if !unicode.IsSpace(rune(text[m[4]])) {
			end = m[4] // the quote or bracket opens the spec
		}
		rest := staticWord.ReplaceAllString(text[end:], "")
		// The ( of require("x") would otherwise cut the spec down to nothing.
		text = strings.TrimLeft(rest, "( ")
	}
	if kw == "mod" { // Rust mod x; names its file the way self::x does
		text = "self::" + text
	}
	if csharpAlias.MatchString(text) { // using Json = Newtonsoft.Json;
		_, text, _ = strings.Cut(text, "=")
		text = strings.TrimSpace(text)
	}
	quoted := text != "" && strings.ContainsAny(text[:1], `"'`)
	isRelative := kw == "require_relative" || strings.HasPrefix(kw, "#") && quoted
	var raw []string
	if kw == "from" && strings.Contains(text, " import ") {
		raw = pythonFrom(text)
	} else {
		raw = expandGroups(text)
	}
	kind := plain
	if isRelative {
		kind = relative
	}
	var out []spec
	for _, r := range raw {
		if loc := specEnd.FindStringIndex(r); loc != nil {
			r = r[:loc[0]]
		}
		r = strings.TrimPrefix(strings.Trim(r, `"'<>`), "package:")
		if r != "" {
			out = append(out, spec{r, kind})
		}
	}
	return out
}

// expandGroups is a::{b, c::{d, e}} as a::b, a::c::d, a::c::e; text with
// no group as is.
func expandGroups(text string) []string {
	start := strings.Index(text, "{")
	if start < 0 {
		return []string{text}
	}
	var items []string
	depth, itemStart := 0, start+1
scan:
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case ',':
			if depth == 1 {
				items = append(items, text[itemStart:i])
				itemStart = i + 1
			}
		case '}':
			depth--
			if depth == 0 {
				items = append(items, text[itemStart:i])
				break scan
			}
		}
	}
	var out []string
	for _, item := range items {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, expandGroups(text[:start]+item)...)
		}
	}
	return out
}

// pythonFrom is "pkg import a, b as c" as pkg.a and pkg.b, and ". import x"
// as .x. A name that is no module falls back to its package later.
func pythonFrom(text string) []string {
	pkg, names, _ := strings.Cut(text, " import ")
	pkg = strings.TrimSpace(pkg)
	joiner := "."
	if strings.HasSuffix(pkg, ".") {
		joiner = ""
	}
	var out []string
	for name := range strings.SplitSeq(strings.Trim(strings.TrimSpace(names), "() "), ",") {
		name, _, _ = strings.Cut(name, " as ")
		switch name = strings.TrimSpace(name); name {
		case "":
		case "*":
			out = append(out, pkg)
		default:
			out = append(out, pkg+joiner+name)
		}
	}
	return out
}

// snakeCase is an Elixir module as its file name: MyApp.HTTPClient is
// my_app.http_client.
func snakeCase(name string) string {
	var b strings.Builder
	for i, r := range name {
		if i > 0 && unicode.IsUpper(r) {
			prev := rune(name[i-1])
			next := rune(0)
			if i+1 < len(name) {
				next = rune(name[i+1])
			}
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || unicode.IsUpper(prev) && unicode.IsLower(next) {
				b.WriteByte('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// externalName is the package an unresolved spec names: @scope/name, a
// platform's two segments, a domain-rooted three, else its first.
func externalName(p string, isDotted bool) string {
	var parts []string
	for part := range strings.SplitSeq(p, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return p
	}
	n := 1
	switch {
	case strings.HasPrefix(p, "@") || isDotted && platformRoots[parts[0]]:
		n = 2
	case isDotted && domainRoots[parts[0]] || !isDotted && strings.Contains(parts[0], "."):
		n = 3
	}
	sep := "/"
	if isDotted {
		sep = "."
	}
	return strings.Join(parts[:min(n, len(parts))], sep)
}
