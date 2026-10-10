package classify

// CommandGrammar is how the gate classifier reads a command line. A line it
// can't read with certainty (a pipe, ||, a subshell, command substitution)
// holds no gate.
type CommandGrammar struct {
	// Wrappers run the rest of the line after any K=V words; never one that
	// loads env (dotenv --), which the gate needs.
	Wrappers []string
	// ToolRunners run the word after them as a tool; the project's own copy
	// is what counts.
	ToolRunners []string
	References  []Reference
	// ScriptRunners take package scripts as every non-flag argument.
	ScriptRunners []string
	// FanOutFlags on a reference, and FanOutCommands whole, run across
	// workspace packages, which run-checks covers per subproject already.
	FanOutFlags    []string
	FanOutCommands []string
}

// Reference is a command prefix that runs another task of Provider: the
// word after it, or Task when set. With Shorthand, a word that names no such
// task is a tool instead (pnpm eslint).
type Reference struct {
	Prefix    string
	Provider  string
	Task      string
	Shorthand bool
}

var Grammar = CommandGrammar{
	Wrappers:    []string{"env", "cross-env"},
	ToolRunners: []string{"npx", "pnpm exec", "pnpm dlx", "bunx", "yarn dlx", "yarn exec", "npm exec", "uv run", "pipx run", "bundle exec", "poetry run", "go tool"},
	References: []Reference{
		{Prefix: "npm run", Provider: "package-scripts"},
		{Prefix: "npm run-script", Provider: "package-scripts"},
		{Prefix: "npm test", Provider: "package-scripts", Task: "test"},
		{Prefix: "npm t", Provider: "package-scripts", Task: "test"},
		{Prefix: "pnpm run", Provider: "package-scripts"},
		{Prefix: "pnpm", Provider: "package-scripts", Shorthand: true},
		{Prefix: "yarn run", Provider: "package-scripts"},
		{Prefix: "yarn", Provider: "package-scripts", Shorthand: true},
		{Prefix: "bun run", Provider: "package-scripts"},
		{Prefix: "node --run", Provider: "package-scripts"},
		{Prefix: "make", Provider: "make"},
		{Prefix: "just", Provider: "just"},
		{Prefix: "rake", Provider: "rake"},
		{Prefix: "bin/rake", Provider: "rake"},
		{Prefix: "pdm run", Provider: "pdm", Shorthand: true},
		{Prefix: "poe", Provider: "poe"},
		{Prefix: "composer run", Provider: "composer"},
		{Prefix: "composer run-script", Provider: "composer"},
		{Prefix: "task", Provider: "taskfile"},
		{Prefix: "pre-commit run", Provider: "pre-commit"},
	},
	ScriptRunners:  []string{"run-s", "run-p", "npm-run-all", "npm-run-all2"},
	FanOutFlags:    []string{"-r", "--recursive", "--filter", "-F", "--workspaces", "--ws", "-ws", "--workspace"},
	FanOutCommands: []string{"turbo run", "turbo", "nx run-many", "nx affected", "lerna run", "yarn workspaces foreach", "pnpm -r"},
}
