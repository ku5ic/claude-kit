package extract

import (
	"regexp"
	"slices"
	"strings"
)

// Body is the shell a task runs. PerLine: each line runs in a shell of its
// own (make and just recipes), so a cd or export ends with its line.
type Body struct {
	Text    string
	PerLine bool
}

// Bodies maps each task an extractor finds to the shell it runs, for the
// gate classifier. Only extractors whose manifest holds commands have
// bodies: json_keys and toml_keys (script strings), make_targets and
// just_recipes (recipes, with prerequisites and dependencies first, as
// "make <dep>" and "just <dep>" lines). Any other extractor has none.
func Bodies(name, file, arg string) map[string]Body {
	switch name {
	case "json_keys":
		return stringValues(GetPath(readJSON(file), arg))
	case "toml_keys":
		return stringValues(GetPath(readTOML(file), arg))
	case "make_targets":
		return makeBodies(file)
	case "just_recipes":
		return justBodies(file)
	}
	return nil
}

// stringValues maps a table's keys to their command: a string, a list of
// strings joined with spaces (a cargo alias), or a table's cmd or shell
// string (poe, pdm). Anything else has no body.
func stringValues(value any) map[string]Body {
	m, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	out := map[string]Body{}
	for key, v := range m {
		switch v := v.(type) {
		case string:
			out[key] = Body{Text: v}
		case []any:
			if words := strings_(v); len(words) == len(v) && len(words) > 0 {
				out[key] = Body{Text: strings.Join(words, " ")}
			}
		case map[string]any:
			for _, field := range []string{"cmd", "shell"} {
				if s, ok := v[field].(string); ok {
					out[key] = Body{Text: s}
					break
				}
			}
		}
	}
	return out
}

var (
	makeVar    = regexp.MustCompile(`\$[({]MAKE[)}]`)
	makeAssign = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*[:?]?=[[:space:]]*(.*)$`)
	makeRef    = regexp.MustCompile(`\$[({]([A-Za-z_][A-Za-z0-9_]*)[)}]`)
)

var oneShell = regexp.MustCompile(`^\.ONESHELL\s*:`)

// makeBodies reads each explicit target's prerequisites and recipe. Recipe
// lines lose their @, -, and + prefixes, $(MAKE) reads as make, a simple
// variable the Makefile sets ($(RUN), ${RUN}) reads as its value, and
// backslash continuations are joined. Each line runs in its own shell
// unless the Makefile declares .ONESHELL.
func makeBodies(file string) map[string]Body {
	lines := readLines(file)
	perLine := !slices.ContainsFunc(lines, oneShell.MatchString)
	vars := map[string]string{}
	for _, line := range lines {
		if m := makeAssign.FindStringSubmatch(line); m != nil {
			vars[m[1]] = strings.TrimSpace(m[2])
		}
	}
	// An unset variable stays a $(...) the classifier won't read.
	expand := func(line string) string {
		return makeRef.ReplaceAllStringFunc(makeVar.ReplaceAllString(line, "make"), func(ref string) string {
			if v, ok := vars[makeRef.FindStringSubmatch(ref)[1]]; ok {
				return v
			}
			return ref
		})
	}
	out := map[string]Body{}
	for i := 0; i < len(lines); i++ {
		if !makeTarget.MatchString(lines[i]) {
			continue
		}
		name, rest, _ := strings.Cut(lines[i], ":")
		name = strings.TrimSpace(name)
		var body []string
		for dep := range strings.FieldsSeq(strings.SplitN(rest, ";", 2)[0]) {
			// | only separates order-only prerequisites, which still run.
			if dep != "|" {
				body = append(body, "make "+dep)
			}
		}
		for i+1 < len(lines) && strings.HasPrefix(lines[i+1], "\t") {
			i++
			line := strings.TrimLeft(strings.TrimPrefix(lines[i], "\t"), "@-+")
			for strings.HasSuffix(line, "\\") && i+1 < len(lines) {
				i++
				line = strings.TrimRight(strings.TrimSuffix(line, "\\"), " \t") + " " + strings.TrimSpace(lines[i])
			}
			body = append(body, expand(line))
		}
		out[name] = Body{Text: strings.Join(body, "\n"), PerLine: perLine}
	}
	return out
}

var shellShebang = regexp.MustCompile(`^#!\s*(/usr/bin/env\s+(-S\s+)?)?(\S*/)?(ba|z)?sh(\s|$)`)

var justVar = regexp.MustCompile(`\{\{[^}]*\}\}`)

var justHeader = regexp.MustCompile(`^@?([A-Za-z_][A-Za-z0-9_-]*)([^:=]*):([^=].*|)$`)

// justBodies reads each recipe's dependencies and indented body. Body
// lines lose their @ and - prefixes. Recipes with parameters keep only
// their dependencies' and body's words; the parameters aren't filled in.
// Each line runs in its own shell, except in a shebang recipe, which is one
// script: a sh or bash one is read whole, any other interpreter's not at all.
func justBodies(file string) map[string]Body {
	lines := readLines(file)
	out := map[string]Body{}
	for i := 0; i < len(lines); i++ {
		m := justHeader.FindStringSubmatch(lines[i])
		if m == nil || strings.HasPrefix(lines[i], " ") || strings.HasPrefix(lines[i], "\t") {
			continue
		}
		var deps, body []string
		for dep := range strings.FieldsSeq(m[3]) {
			deps = append(deps, "just "+dep)
		}
		for i+1 < len(lines) && (strings.HasPrefix(lines[i+1], " ") || strings.HasPrefix(lines[i+1], "\t")) {
			i++
			// {{var}} is just's interpolation: a value the kit can't know.
			body = append(body, justVar.ReplaceAllString(strings.TrimLeft(strings.TrimSpace(lines[i]), "@-"), "$$JUST_VAR"))
		}
		perLine := true
		if len(body) > 0 && strings.HasPrefix(body[0], "#!") {
			perLine = false
			if !shellShebang.MatchString(body[0]) {
				body = nil
			}
		}
		out[m[1]] = Body{Text: strings.Join(append(deps, body...), "\n"), PerLine: perLine}
	}
	return out
}

func readLines(file string) []string {
	var lines []string
	EachLine(file, func(line string) { lines = append(lines, line) })
	return lines
}
