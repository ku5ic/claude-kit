package classify

import "github.com/ku5ic/claude-kit/go/internal/sources"

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

// Reference is a command prefix that runs another task of Provider.
type Reference struct {
	sources.Prefix
	Provider string
}

// references are every task provider's prefixes.
func references() []Reference {
	var refs []Reference
	for _, tp := range sources.TaskProviders {
		for _, p := range tp.Prefixes {
			refs = append(refs, Reference{p, tp.Name})
		}
	}
	return refs
}

var Grammar = CommandGrammar{
	Wrappers:       []string{"env", "cross-env"},
	ToolRunners:    []string{"npx", "pnpm exec", "pnpm dlx", "bunx", "yarn dlx", "yarn exec", "npm exec", "uv run", "pipx run", "bundle exec", "poetry run", "go tool"},
	References:     references(),
	ScriptRunners:  []string{"run-s", "run-p", "npm-run-all", "npm-run-all2"},
	FanOutFlags:    []string{"-r", "--recursive", "--filter", "-F", "--workspaces", "--ws", "-ws", "--workspace"},
	FanOutCommands: []string{"turbo run", "turbo", "nx run-many", "nx affected", "lerna run", "yarn workspaces foreach", "pnpm -r"},
}
