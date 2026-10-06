package tools

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/ku5ic/claude-kit/go/internal/bashguard"
	"github.com/ku5ic/claude-kit/go/internal/extract"
)

// Derived is what a project's own invocation of a tool adds to the
// file-scoped command: its env assignments and the allow-listed flags
// (Carry), with where they came from. The project's paths, globs, and any
// flag not on the list never carry: the check runs on the edited files, and
// never writes.
type Derived struct {
	Env    []string
	Flags  []string
	Source string
}

// Runners wrap a tool without changing it: npx eslint, uv run ruff.
var runners = map[string][][]string{
	"npx":    {{"npx"}},
	"pnpm":   {{"pnpm", "exec"}, {"pnpm", "dlx"}, {"pnpm"}},
	"yarn":   {{"yarn", "run"}, {"yarn", "exec"}, {"yarn", "dlx"}, {"yarn"}},
	"npm":    {{"npm", "exec", "--"}, {"npm", "exec"}},
	"bunx":   {{"bunx"}},
	"bun":    {{"bun", "x"}, {"bun", "run"}},
	"uv":     {{"uv", "run"}},
	"poetry": {{"poetry", "run"}},
	"pdm":    {{"pdm", "run"}},
	"pipenv": {{"pipenv", "run"}},
	"bundle": {{"bundle", "exec"}},
	"go":     {{"go", "run"}},
}

var envWord = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// carriedEnv is every env name a derived check may carry. Repo scripts are
// untrusted: GOFLAGS=-toolexec, NODE_OPTIONS=--require, or PYTHONPATH would
// run repo code under tools that otherwise don't.
var carriedEnv = map[string]bool{"CI": true, "TZ": true, "NODE_ENV": true, "DEBUG": true, "FORCE_COLOR": true, "NO_COLOR": true}

// match finds the adapter's tool in one parsed command: past env
// assignments, env/cross-env, and a runner (with the runner's own
// options), the tool's name, then its required subcommand.
func (a Adapter) match(call bashguard.Call) (Derived, bool) {
	words := make([]string, 0, len(call.Words))
	for _, w := range call.Words {
		words = append(words, w.Value)
	}
	var env []string
	// Only literal values: NODE_OPTIONS="${NODE_OPTIONS:-}" would reach env
	// as that text, not its expansion.
	addEnv := func(assign string) {
		name, _, _ := strings.Cut(assign, "=")
		if carriedEnv[name] && !strings.ContainsAny(assign, "$`") {
			env = append(env, assign)
		}
	}
	for _, a := range call.Assigns {
		addEnv(a)
	}
	for len(words) > 0 && (words[0] == "env" || words[0] == "cross-env") {
		words = words[1:]
		for len(words) > 0 && envWord.MatchString(words[0]) {
			addEnv(words[0])
			words = words[1:]
		}
	}
	if len(words) == 0 {
		return Derived{}, false
	}
	for _, form := range runners[filepath.Base(words[0])] {
		if len(words) > len(form) && slices.Equal(append([]string{filepath.Base(words[0])}, words[1:len(form)]...), form) {
			words = words[len(form):]
			// The runner's own options: uv run --no-dev ruff.
			for len(words) > 1 && strings.HasPrefix(words[0], "-") {
				words = words[1:]
			}
			break
		}
	}
	if len(words) == 0 || filepath.Base(words[0]) != a.Bin {
		return Derived{}, false
	}
	words = words[1:]
	if len(a.Sub) > 0 {
		if len(words) == 0 || !slices.Contains(a.Sub, words[0]) {
			return Derived{}, false
		}
		words = words[1:]
	}
	d := Derived{Env: env}
	for i := 0; i < len(words); i++ {
		flag, _, hasValue := strings.Cut(words[i], "=")
		takesValue, carried := a.Carry[flag]
		if !carried {
			continue
		}
		d.Flags = append(d.Flags, words[i])
		if takesValue && !hasValue && i+1 < len(words) {
			i++
			d.Flags = append(d.Flags, words[i])
		}
	}
	return d, true
}

// fromScript tries every command of one script body.
func (a Adapter) fromScript(body, source string) (Derived, bool) {
	for _, seg := range bashguard.Segments(body) {
		for _, call := range seg.Calls {
			if d, ok := a.match(call); ok {
				d.Source = source
				return d, true
			}
		}
	}
	return Derived{}, false
}

// Derive finds the project's own invocation of the tool, from the run
// directory's package.json scripts, then its Makefile, then its justfile,
// then the repo's CI workflow steps that run in that directory. Scripts
// named for fixing, watching, or UIs are skipped: their flags aren't a
// check's.
func (a Adapter) Derive(dir, root string) (Derived, bool) {
	for _, s := range packageScripts(dir) {
		if d, ok := a.fromScript(s.body, "package.json scripts."+s.name); ok {
			return d, true
		}
	}
	for _, r := range makeRecipes(filepath.Join(dir, "Makefile")) {
		if d, ok := a.fromScript(r.body, "Makefile target "+r.name); ok {
			return d, true
		}
	}
	for _, r := range justRecipes(filepath.Join(dir, "justfile")) {
		if d, ok := a.fromScript(r.body, "justfile recipe "+r.name); ok {
			return d, true
		}
	}
	// CI jobs are peers, so a bare `eslint .` in one doesn't hide another
	// job's --max-warnings: the first step carrying flags wins.
	var first Derived
	found := false
	for _, s := range ciSteps(root, dir) {
		d, ok := a.fromScript(s.body, s.name)
		if !ok {
			continue
		}
		if len(d.Flags) > 0 {
			return d, true
		}
		if !found {
			first, found = d, true
		}
	}
	return first, found
}

type script struct{ name, body string }

var notACheck = regexp.MustCompile(`(^|:)(fix|format|watch|dev|ui|coverage)($|:)|:fix$`)

// packageScripts are package.json's scripts in file order, minus the ones
// that fix, watch, or serve.
func packageScripts(dir string) []script {
	file := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return nil
	}
	var out []script
	for _, name := range extract.JSONKeys(file, ".scripts") {
		if !notACheck.MatchString(name) {
			out = append(out, script{name, pkg.Scripts[name]})
		}
	}
	return out
}

var (
	makeAssign = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*[:?]?=[[:space:]]*(.*)$`)
	makeRef    = regexp.MustCompile(`\$[({]([A-Za-z_][A-Za-z0-9_]*)[)}]`)
	makeTarget = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9_.-]*)[[:space:]]*:([^=]|$)`)
)

// makeRecipes are each target's recipe lines, with simple variables
// ($(RUN), ${RUN}) expanded and the @, -, + prefixes dropped.
func makeRecipes(path string) []script {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	vars := map[string]string{}
	var out []script
	target := ""
	for line := range strings.SplitSeq(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "\t") && target != "":
			body := strings.TrimLeft(strings.TrimSpace(line), "@-+")
			body = makeRef.ReplaceAllStringFunc(body, func(ref string) string { return vars[makeRef.FindStringSubmatch(ref)[1]] })
			if !notACheck.MatchString(target) {
				out = append(out, script{target, body})
			}
		case makeAssign.MatchString(line):
			m := makeAssign.FindStringSubmatch(line)
			vars[m[1]] = strings.TrimSpace(m[2])
		case makeTarget.MatchString(line):
			target = makeTarget.FindStringSubmatch(line)[1]
		default:
			if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
				continue
			}
			target = ""
		}
	}
	return out
}

var (
	justHeader = regexp.MustCompile(`^@?([A-Za-z_][A-Za-z0-9_-]*)([[:space:]][^:=]*)?:([^=]|$)`)
	justInterp = regexp.MustCompile(`\{\{[^}]*\}\}`)
)

// justRecipes are each recipe's indented body lines, {{...}} dropped.
func justRecipes(path string) []script {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []script
	recipe := ""
	for line := range strings.SplitSeq(string(data), "\n") {
		switch {
		case (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && recipe != "":
			body := strings.TrimLeft(strings.TrimSpace(line), "@-")
			if !notACheck.MatchString(recipe) {
				out = append(out, script{recipe, justInterp.ReplaceAllString(body, "")})
			}
		case justHeader.MatchString(line):
			recipe = justHeader.FindStringSubmatch(line)[1]
		default:
			recipe = ""
		}
	}
	return out
}

// ciSteps are the run: steps of .github/workflows/*.yml whose
// working-directory (the step's, else the job's defaults) is dir.
func ciSteps(root, dir string) []script {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return nil
	}
	workflows, _ := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.y*ml"))
	var out []script
	for _, wf := range workflows {
		data, err := os.ReadFile(wf)
		if err != nil {
			continue
		}
		var doc struct {
			Jobs map[string]struct {
				Defaults struct {
					Run struct {
						WorkingDirectory string `yaml:"working-directory"`
					} `yaml:"run"`
				} `yaml:"defaults"`
				Steps []struct {
					Name             string `yaml:"name"`
					Run              string `yaml:"run"`
					WorkingDirectory string `yaml:"working-directory"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if yaml.Unmarshal(data, &doc) != nil {
			continue
		}
		// Sorted: a map's order would change which job's flags carry.
		for _, name := range slices.Sorted(maps.Keys(doc.Jobs)) {
			job := doc.Jobs[name]
			for _, step := range job.Steps {
				wd := step.WorkingDirectory
				if wd == "" {
					wd = job.Defaults.Run.WorkingDirectory
				}
				if step.Run != "" && filepath.Clean(wd) == rel {
					out = append(out, script{".github/workflows/" + filepath.Base(wf) + " step " + step.Name, step.Run})
				}
			}
		}
	}
	return out
}
