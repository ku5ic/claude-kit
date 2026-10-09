package tools

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/extract"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// Ecosystem is where an adapter's declared dependency is looked up.
type Ecosystem string

const (
	JS     Ecosystem = "js"
	Python Ecosystem = "python"
	Ruby   Ecosystem = "ruby"
)

// Adapter is one file-scoped check. It claims an edited file when the
// extension matches and the project uses the tool: a config file (Signals)
// or a TOML table (TOML) found walking up from the file, or the tool
// declared as a dependency (Packages) in the nearest manifest that names
// it. The directory of whatever claimed it is where the check runs.
type Adapter struct {
	Name     string
	Ext      []string
	Bin      string
	Cmd      string // {bin} whole word; a word holding {files} or {dirs} repeats per item
	Signals  []string
	TOML     string // "<file> <dotted path>"
	Deps     Ecosystem
	Packages []string
	// TestRunner: when the dependency is declared beside another test
	// runner's, run only the one the package.json test script names (a
	// vitest that only drives Storybook beside a jest test script).
	TestRunner bool
	// Needs: one must also exist from the run directory up to the root.
	Needs []string
	// ExcludeTOML: "<file> <dotted path>" to a regex or regex list; files
	// matching it (relative to the run directory) are dropped, for a tool
	// that checks named files its own exclude covers (mypy).
	ExcludeTOML string
	// LocalOnly skips PATH: a copy from there can't see the project's
	// packages.
	LocalOnly bool
	Builtin   bool

	// Derivation from the project's own scripts (Derive): the subcommand
	// the project's invocation must use (ruff check, golangci-lint run), and
	// the flags carried into the file-scoped command, each mapped to
	// whether it takes a separate value.
	Sub   []string
	Carry map[string]bool

	// Findings parses the output so only findings on changed lines block;
	// nil keeps the whole-file verdict (type checkers, test runners).
	Findings *Findings
}

// Claim is why an adapter claims a file: the directory it runs from and
// the evidence, for kit explain.
type Claim struct {
	Dir string
	Why string
}

// Claims reports whether a claims path (physical, under root).
func (a Adapter) Claims(path, root string) (Claim, bool) {
	base := filepath.Base(path)
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(base), "."))
	if ext == "" || !slices.Contains(a.Ext, ext) {
		return Claim{}, false
	}
	claim, ok := a.detect(filepath.Dir(path), root)
	if !ok {
		return Claim{}, false
	}
	if len(a.Needs) > 0 {
		needed := project.FindUp(claim.Dir, root, a.Needs...)
		if needed == "" {
			return Claim{}, false
		}
		// The signal says where it runs (go.mod); this is the config it reads.
		claim.Why += ", needs " + strings.TrimPrefix(needed, root+"/")
	}
	if a.ExcludeTOML != "" && excluded(claim.Dir, path, a.ExcludeTOML) {
		return Claim{}, false
	}
	return claim, true
}

// HasSignal finds a tool's config walking up from dir to root: one of
// files by name, else a "<file> <dotted path>" TOML table. The TOML walk
// goes past a nearer file without the table, as ruff's lookup does.
func HasSignal(files []string, toml, dir, root string) (Claim, bool) {
	if len(files) > 0 {
		if found := project.FindUp(dir, root, files...); found != "" {
			return Claim{filepath.Dir(found), "config " + strings.TrimPrefix(found, root+"/")}, true
		}
	}
	if toml == "" {
		return Claim{}, false
	}
	file, table, _ := strings.Cut(toml, " ")
	for from := dir; ; {
		found := project.FindUp(from, root, file)
		if found == "" {
			return Claim{}, false
		}
		if extract.TOMLHas(found, table) {
			return Claim{filepath.Dir(found), "[" + strings.TrimPrefix(table, ".") + "] in " + strings.TrimPrefix(found, root+"/")}, true
		}
		if filepath.Dir(found) == root {
			return Claim{}, false
		}
		from = filepath.Dir(filepath.Dir(found))
	}
}

// fromConfig fills what kit.yml already owns: a built-in without signals
// takes those of the formatter of its name, and one without a subcommand
// takes the first check pattern for its bin that names one.
func (a Adapter) fromConfig(cfg *config.Config) Adapter {
	if a.Signals == nil && a.TOML == "" {
		if i := slices.IndexFunc(cfg.Formatters, func(f config.Formatter) bool { return f.Name == a.Name }); i >= 0 {
			a.Signals, a.TOML = cfg.Formatters[i].SignalFiles, cfg.Formatters[i].SignalTOML
		}
	}
	for _, c := range cfg.Checks {
		for _, p := range c.Tools {
			if a.Sub == nil && p.Bin == a.Bin && len(p.Sub) > 0 {
				a.Sub = p.Sub
			}
		}
	}
	return a
}

func (a Adapter) detect(dir, root string) (Claim, bool) {
	if claim, ok := HasSignal(a.Signals, a.TOML, dir, root); ok {
		return claim, true
	}
	if a.Deps != "" {
		for d := dir; ; d = filepath.Dir(d) {
			if pkg, ok := a.declared(d); ok {
				if a.TestRunner && !a.chosenRunner(d) {
					return Claim{}, false
				}
				return Claim{d, "dependency " + pkg + " in " + manifestName(a.Deps, d, root)}, true
			}
			if d == root || d == "/" || !strings.HasPrefix(d, root) {
				break
			}
		}
	}
	return Claim{}, false
}

// declared is the first of the adapter's packages dir's manifest declares.
func (a Adapter) declared(dir string) (string, bool) {
	var deps Deps
	switch a.Deps {
	case JS:
		deps = JSDeps(dir)
	case Python:
		deps = PythonDeps(dir)
	case Ruby:
		deps = RubyDeps(dir)
	}
	for _, pkg := range a.Packages {
		key := pkg
		if a.Deps == Python {
			key = normalize(pkg)
		}
		if deps[key] {
			return pkg, true
		}
	}
	return "", false
}

var testRunners = []string{"jest", "vitest"}

// chosenRunner is false when another test runner is declared in dir too
// and the test script names that one instead.
func (a Adapter) chosenRunner(dir string) bool {
	deps := JSDeps(dir)
	others := 0
	for _, r := range testRunners {
		if r != a.Name && deps[r] {
			others++
		}
	}
	if others == 0 {
		return true
	}
	return slices.Contains(strings.Fields(nonWord.ReplaceAllString(TestScript(dir), " ")), a.Name)
}

var nonWord = regexp.MustCompile(`[^A-Za-z0-9_-]`)

func manifestName(eco Ecosystem, dir, root string) string {
	var names []string
	switch eco {
	case JS:
		names = []string{"package.json"}
	case Python:
		names = []string{"pyproject.toml", "requirements.txt"}
	case Ruby:
		names = []string{"Gemfile.lock"}
	}
	for _, n := range names {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			return strings.TrimPrefix(filepath.Join(dir, n), root+"/")
		}
	}
	return strings.TrimPrefix(dir, root+"/")
}

// excluded is true when path, relative to dir, matches a regex the TOML
// file in dir holds at the dotted path (a string or a list), as mypy's
// exclude matches with re.search.
func excluded(dir, path, spec string) bool {
	file, table, _ := strings.Cut(spec, " ")
	tomlPath := filepath.Join(dir, file)
	rel := strings.TrimPrefix(path, dir+"/")
	patterns := extract.TOMLArray(tomlPath, table)
	if len(patterns) == 0 {
		if v := extract.TOMLString(tomlPath, table); v != "" {
			patterns = []string{v}
		}
	}
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil && re.MatchString(rel) {
			return true
		}
	}
	return false
}

// All is the built-in adapters, with kit.yml's file_checks merged in: an
// entry named like a built-in replaces it, any other is added, and
// disabled_file_checks drops either kind.
func All(cfg *config.Config) []Adapter {
	var out []Adapter
	custom := map[string]bool{}
	for _, fc := range cfg.FileChecks {
		custom[fc.Name] = true
	}
	for _, a := range builtins {
		if !custom[a.Name] && !slices.Contains(cfg.DisabledFileChecks, a.Name) {
			out = append(out, a.fromConfig(cfg))
		}
	}
	for _, fc := range cfg.FileChecks {
		if slices.Contains(cfg.DisabledFileChecks, fc.Name) {
			continue
		}
		out = append(out, Adapter{
			Name: fc.Name, Ext: fc.Ext, Bin: fc.Bin, Cmd: fc.Cmd,
			Signals: fc.SignalFiles, TOML: fc.SignalTOML,
			ExcludeTOML: fc.ExcludeTOML, LocalOnly: fc.LocalOnly,
		})
	}
	return out
}
