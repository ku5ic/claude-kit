package pkgcheck

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
)

var (
	jsExts = []string{".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts", ".vue", ".svelte"}
	// jsImport is a module specifier after from, import, import(, or require(.
	jsImport = regexp.MustCompile(`(?:\bfrom\s*|\bimport\s*\(?\s*|\brequire\s*\(\s*)['"]([^'"\n]+)['"]`)
	// nodeBuiltins import without a node: prefix too.
	nodeBuiltins = []string{"assert", "async_hooks", "buffer", "child_process", "cluster", "console", "constants", "crypto",
		"dgram", "diagnostics_channel", "dns", "domain", "events", "fs", "http", "http2", "https", "inspector", "module", "net",
		"os", "path", "perf_hooks", "process", "punycode", "querystring", "readline", "repl", "stream", "string_decoder", "sys",
		"timers", "tls", "trace_events", "tty", "url", "util", "v8", "vm", "wasi", "worker_threads", "zlib"}
	depFields = []string{"dependencies", "devDependencies", "peerDependencies", "optionalDependencies"}
	// aliasConfigs are the files that map an import prefix to a path.
	aliasConfigs = []string{"tsconfig*.json", "jsconfig*.json", "vite.config.*", "vitest.config.*", "webpack.config.*",
		"next.config.*", "nuxt.config.*", "svelte.config.*", "astro.config.*", "babel.config.*", ".babelrc*"}
)

// Undeclared is each package an edit of path adds an import of that no
// manifest declares: no package.json from path's directory up for JS and
// TS, and neither the standard library nor go.mod (or a go.work module)
// for Go. nil for any other file, under a skipDirs or testdata directory,
// or with no manifest to check against.
func Undeclared(skipDirs []string, path, before, after string) []string {
	for _, dir := range strings.Split(filepath.ToSlash(filepath.Dir(path)), "/") {
		if dir == "testdata" || slices.Contains(skipDirs, dir) {
			return nil
		}
	}
	switch ext := filepath.Ext(path); {
	case ext == ".go":
		return goUndeclared(path, before, after)
	case slices.Contains(jsExts, ext):
		return jsUndeclared(path, before, after)
	}
	return nil
}

func goUndeclared(path, before, after string) []string {
	gomod := fsx.FindUp(filepath.Dir(path), "/", "go.mod")
	if gomod == "" {
		return nil
	}
	mods := modules(gomod)
	if work := fsx.FindUp(filepath.Dir(path), "/", "go.work"); work != "" {
		for _, dir := range directives(work)["use"] {
			mods = append(mods, directives(filepath.Join(filepath.Dir(work), dir, "go.mod"))["module"]...)
		}
	}
	var out []string
	for _, imp := range added(goImports(before), goImports(after)) {
		first, _, _ := strings.Cut(imp, "/")
		if !strings.Contains(first, ".") { // the standard library's paths have no dot
			continue
		}
		if !slices.ContainsFunc(mods, func(m string) bool { return imp == m || strings.HasPrefix(imp, m+"/") }) {
			out = append(out, imp)
		}
	}
	return out
}

// modules is the module path go.mod declares and every module it
// requires or replaces.
func modules(gomod string) []string {
	d := directives(gomod)
	return slices.Concat(d["module"], d["require"], d["replace"])
}

// directives is the first word of each directive in a go.mod or go.work,
// by directive, block form included.
func directives(path string) map[string][]string {
	data, err := os.ReadFile(path)
	out := map[string][]string{}
	if err != nil {
		return out
	}
	block := ""
	for _, line := range strings.Split(string(data), "\n") {
		line, _, _ = strings.Cut(line, "//")
		f := strings.Fields(line)
		switch {
		case len(f) == 0:
		case f[0] == ")":
			block = ""
		case len(f) == 2 && f[1] == "(":
			block = f[0]
		case block != "":
			out[block] = append(out[block], unquote(f[0]))
		case len(f) > 1:
			out[f[0]] = append(out[f[0]], unquote(f[1]))
		}
	}
	return out
}

func unquote(s string) string {
	if u, err := strconv.Unquote(s); err == nil {
		return u
	}
	return s
}

// goImports is src's import paths; nil when src doesn't parse.
func goImports(src string) []string {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.ImportsOnly)
	if err != nil {
		return nil
	}
	var out []string
	for _, spec := range f.Imports {
		out = append(out, unquote(spec.Path.Value))
	}
	return out
}

func jsUndeclared(path, before, after string) []string {
	declared := map[string]bool{}
	var roots []string
	for dir := range fsx.Up(filepath.Dir(path), "/") {
		data, err := os.ReadFile(filepath.Join(dir, "package.json"))
		if err != nil {
			continue
		}
		roots = append(roots, dir)
		var manifest map[string]json.RawMessage
		if json.Unmarshal(data, &manifest) != nil {
			return nil // a manifest that doesn't parse can't say what's missing
		}
		var name string
		_ = json.Unmarshal(manifest["name"], &name)
		declared[name] = true
		for _, field := range depFields {
			var deps map[string]any
			_ = json.Unmarshal(manifest[field], &deps)
			for dep := range deps {
				declared[dep] = true
			}
		}
	}
	if len(roots) == 0 {
		return nil
	}
	var out []string
	for _, spec := range added(jsSpecs(before), jsSpecs(after)) {
		name := jsPackage(spec)
		if name == "" || declared[name] || slices.Contains(nodeBuiltins, name) || aliased(roots, name) || slices.Contains(out, name) {
			continue
		}
		out = append(out, name)
	}
	return out
}

func jsSpecs(src string) []string {
	var out []string
	for _, m := range jsImport.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	return out
}

// jsPackage is the package a module specifier names, or "" for a relative
// or absolute path, a protocol (node:, virtual:), or an alias prefix.
func jsPackage(spec string) string {
	if strings.ContainsAny(spec[:1], "./~#$") || strings.Contains(spec, ":") {
		return ""
	}
	parts := strings.Split(spec, "/")
	if strings.HasPrefix(spec, "@") {
		if len(parts) < 2 || parts[0] == "@" {
			return "" // "@/x" is a path alias
		}
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

// aliased is true when a project root, or its src, holds a file or
// directory named like the package (a baseUrl alias), or a root's bundler
// or tsconfig quotes its first path segment (a paths or resolve.alias key).
func aliased(roots []string, name string) bool {
	head, _, _ := strings.Cut(name, "/")
	for _, root := range roots {
		for _, pattern := range aliasConfigs {
			matches, _ := filepath.Glob(filepath.Join(root, pattern))
			for _, f := range matches {
				data, err := os.ReadFile(f)
				if err != nil {
					continue
				}
				for _, q := range []string{`"`, `'`} {
					if strings.Contains(string(data), q+head+"/") || strings.Contains(string(data), q+head+q) {
						return true
					}
				}
			}
		}
		for _, dir := range []string{root, filepath.Join(root, "src")} {
			base := filepath.Join(dir, name)
			if _, err := os.Lstat(base); err == nil {
				return true
			}
			if matches, _ := filepath.Glob(base + ".*"); len(matches) > 0 {
				return true
			}
		}
	}
	return false
}

// added is each item of after that before lacks, once.
func added(before, after []string) []string {
	var out []string
	for _, a := range after {
		if !slices.Contains(before, a) && !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	return out
}
