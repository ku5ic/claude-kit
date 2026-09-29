package tools

import "regexp"

var jsExt = []string{"js", "jsx", "ts", "tsx", "mjs", "cjs", "mts", "cts"}

// flags maps each carried flag to whether it takes a separate value.
func flags(valued []string, bare ...string) map[string]bool {
	m := map[string]bool{}
	for _, f := range valued {
		m[f] = true
	}
	for _, f := range bare {
		m[f] = false
	}
	return m
}

// builtins are the file-scoped checks every project gets when it uses the
// tool. None writes: no --fix, no --write, no new snapshots (CI mode);
// stylelint and markdownlint-cli2 still apply a config's own fix setting,
// since neither has a command-line override.
//
// Carry lists the flags a project's own invocation passes on: those that
// change what is checked or how strictly (config, rule and project
// selection, warning thresholds; --cache, whose cache file the project
// already keeps). Nothing that writes source or picks output files carries.
var builtins = []Adapter{
	{
		Name: "eslint", Ext: jsExt, Bin: "eslint", Cmd: "{bin} {files}",
		Findings: &Findings{Header: regexp.MustCompile(`^(?P<file>/.*\S)\s*$`), Item: regexp.MustCompile(`^\s+(?P<line>\d+):\d+\s+(?:error|warning)\s`)},
		Signals: []string{"eslint.config.js", "eslint.config.mjs", "eslint.config.cjs", "eslint.config.ts", "eslint.config.mts", "eslint.config.cts",
			".eslintrc.js", ".eslintrc.cjs", ".eslintrc.json", ".eslintrc.yml", ".eslintrc.yaml", ".eslintrc"},
		Carry: flags([]string{"-c", "--config", "--max-warnings", "--rule", "--ext", "--resolve-plugins-relative-to", "--ignore-path", "--cache-location", "--cache-strategy"},
			"--cache", "--no-eslintrc", "--no-inline-config", "--report-unused-disable-directives"),
	},
	{
		// --no-errors-on-unmatched: files biome.json ignores are otherwise an error.
		Name: "biome", Ext: []string{"js", "jsx", "ts", "tsx", "json", "jsonc", "css"}, Bin: "biome",
		Cmd: "{bin} check --no-errors-on-unmatched {files}", Signals: []string{"biome.json", "biome.jsonc"},
		Sub: []string{"check", "lint", "ci"}, Carry: flags([]string{"--config-path"}),
		// A format or import-order diagnostic has no line: the whole file.
		Findings: &Findings{
			Item:   regexp.MustCompile(`^(?P<file>[^\s:]+)(?::(?P<line>\d+):\d+)?\s+(?:lint|format|assist|parse|organizeImports)\b`),
			Hidden: regexp.MustCompile(`(?i)diagnostics not shown`),
		},
	},
	{
		Name: "stylelint", Ext: []string{"css", "scss"}, Bin: "stylelint", Cmd: "{bin} --allow-empty-input -f unix {files}",
		Findings: lines(`^(?P<file>.+?):(?P<line>\d+):\d+: `),
		Signals: []string{"stylelint.config.js", "stylelint.config.mjs", "stylelint.config.cjs", ".stylelintrc", ".stylelintrc.json",
			".stylelintrc.js", ".stylelintrc.cjs", ".stylelintrc.yml", ".stylelintrc.yaml"},
		Carry: flags([]string{"-c", "--config", "--config-basedir", "--max-warnings", "--ignore-path", "--custom-syntax", "--cache-location"}, "--cache"),
	},
	{
		// CI=1: outside CI, vitest writes new snapshots.
		Name: "vitest", Ext: jsExt, Bin: "vitest", Cmd: "env CI=1 {bin} related --run --passWithNoTests {files}",
		Deps: JS, Packages: []string{"vitest"}, TestRunner: true,
		Carry: flags([]string{"--project", "-c", "--config", "--environment", "-r", "--root"}),
	},
	{
		// --ci: fail on a missing snapshot instead of writing it.
		Name: "jest", Ext: jsExt, Bin: "jest", Cmd: "{bin} --ci --findRelatedTests --passWithNoTests {files}",
		Deps: JS, Packages: []string{"jest"}, TestRunner: true,
		Carry: flags([]string{"-c", "--config", "--testEnvironment", "-w", "--maxWorkers"}),
	},
	{
		// --no-fix: a project's `fix = true` would otherwise rewrite the files.
		Name: "ruff", Ext: []string{"py"}, Bin: "ruff", Cmd: "{bin} check --no-fix --force-exclude --output-format concise {files}",
		Findings: lines(`^(?P<file>.+?):(?P<line>\d+):\d+: `),
		Signals:  []string{"ruff.toml", ".ruff.toml"}, TOML: "pyproject.toml .tool.ruff", Deps: Python, Packages: []string{"ruff"},
		Sub: []string{"check"}, Carry: flags([]string{"--config", "--select", "--extend-select", "--ignore", "--target-version", "--line-length"}),
	},
	{
		// mypy checks a file named on its command line even when its
		// exclude covers it, so the exclude is applied first.
		Name: "mypy", Ext: []string{"py"}, Bin: "mypy", Cmd: "{bin} {files}",
		Signals: []string{"mypy.ini", ".mypy.ini"}, TOML: "pyproject.toml .tool.mypy", Deps: Python, Packages: []string{"mypy"},
		ExcludeTOML: "pyproject.toml .tool.mypy.exclude", LocalOnly: true,
		Carry: flags([]string{"--config-file", "--python-version"}, "--strict", "--ignore-missing-imports", "--check-untyped-defs"),
	},
	{
		Name: "pyright", Ext: []string{"py"}, Bin: "pyright", Cmd: "{bin} {files}",
		Signals: []string{"pyrightconfig.json"}, TOML: "pyproject.toml .tool.pyright", Deps: Python, Packages: []string{"pyright"},
		LocalOnly: true, Carry: flags([]string{"-p", "--project", "--pythonversion", "--level"}),
	},
	{
		// Runs from the module (go.mod), where packages load; golangci-lint
		// finds its config by walking up. --fix=false against issues.fix.
		Name: "golangci-lint", Ext: []string{"go"}, Bin: "golangci-lint", Cmd: "{bin} run --fix=false --max-issues-per-linter=0 --max-same-issues=0 {dirs}",
		Findings: lines(`^(?P<file>[^\s:]+\.go):(?P<line>\d+)(?::\d+)?: `),
		Signals:  []string{"go.mod"}, Needs: []string{".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"},
		Sub: []string{"run"}, Carry: flags([]string{"-c", "--config", "-E", "--enable", "-D", "--disable", "--build-tags", "--timeout"}),
	},
	{Name: "go-vet", Ext: []string{"go"}, Bin: "go", Cmd: "{bin} vet {dirs}", Signals: []string{"go.mod"}, Sub: []string{"vet"}, Carry: flags([]string{"-tags"})},
	{
		Name: "rubocop", Ext: []string{"rb"}, Bin: "rubocop", Cmd: "{bin} --force-exclusion --format emacs {files}",
		Findings: lines(`^(?P<file>.+?):(?P<line>\d+):\d+: [A-Z]: `),
		Signals:  []string{".rubocop.yml"}, Deps: Ruby, Packages: []string{"rubocop"},
		Carry: flags([]string{"-c", "--config", "--only", "--except"}),
	},
	{
		// Its arguments are globs (":" marks a literal path); --no-globs
		// drops the config's own, which would lint the whole repo.
		Name: "markdownlint", Ext: []string{"md"}, Bin: "markdownlint-cli2", Cmd: "{bin} --no-globs :{files}",
		Findings: lines(`^(?P<file>[^\s:]+):(?P<line>\d+)(?::\d+)?\s`),
		Signals: []string{".markdownlint-cli2.jsonc", ".markdownlint-cli2.yaml", ".markdownlint-cli2.cjs", ".markdownlint-cli2.mjs",
			".markdownlint.json", ".markdownlint.jsonc", ".markdownlint.yaml", ".markdownlint.yml"},
		Carry: flags([]string{"--config"}),
	},
	{
		Name: "yamllint", Ext: []string{"yml", "yaml"}, Bin: "yamllint", Cmd: "{bin} -f parsable {files}",
		Findings: lines(`^(?P<file>.+?):(?P<line>\d+):\d+: \[`),
		Signals:  []string{".yamllint", ".yamllint.yml", ".yamllint.yaml"},
		Carry:    flags([]string{"-c", "--config-file", "-d", "--config-data"}, "-s", "--strict"),
	},
	{Name: "tofu-fmt", Ext: []string{"tf", "tfvars"}, Bin: "tofu", Cmd: "{bin} fmt -check {files}", Signals: []string{".terraform.lock.hcl"}, Sub: []string{"fmt"}},
	{
		Name: "shellcheck", Ext: []string{"sh", "bash"}, Bin: "shellcheck", Cmd: "{bin} -x -f gcc {files}", Signals: []string{".shellcheckrc"},
		Findings: lines(`^(?P<file>.+?):(?P<line>\d+):\d+: (?:error|warning|note|info|style): `),
		Carry:    flags([]string{"-P", "--source-path", "-S", "--severity", "-e", "--exclude", "-s", "--shell", "-o", "--enable"}),
	},
}

func init() {
	for i := range builtins {
		builtins[i].Builtin = true
	}
}
