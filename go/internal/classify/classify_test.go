package classify

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

func kitConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, _, err := config.Load(config.Paths{Base: "../../../kit.yml", Overlay: filepath.Join(t.TempDir(), "none.yml")})
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// scripts is the package.json the lookup answers for: lint, test, typecheck.
func scripts(provider, dir, name string) bool {
	return provider == "package-scripts" && dir == "" && (name == "lint" || name == "test" || name == "typecheck")
}

// describe renders a result compactly: "gate:lint:eslint", "ref:make:web:lint",
// "fanout", "cd", "export", "other", or "opaque:<why>".
func describe(r Result) string {
	if r.Opaque != "" {
		return "opaque:" + r.Opaque
	}
	var parts []string
	for _, c := range r.Commands {
		switch c.Kind {
		case Gate:
			parts = append(parts, "gate:"+c.Slot+":"+c.Tool)
		case Ref:
			for _, ref := range c.Refs {
				s := "ref:" + ref.Provider + ":" + ref.Dir + ":" + ref.Name
				if len(ref.Args) > 0 {
					s += ":" + strings.Join(ref.Args, " ")
				}
				parts = append(parts, s)
			}
		case FanOut:
			parts = append(parts, "fanout")
		case Cd:
			parts = append(parts, "cd")
		case Export:
			parts = append(parts, "export")
		default:
			parts = append(parts, "other")
		}
	}
	return strings.Join(parts, " ")
}

func TestBody(t *testing.T) {
	cfg := kitConfig(t)
	for _, c := range []struct{ body, want string }{
		// Plain tools, by each slot's patterns.
		{"eslint .", "gate:lint:eslint"},
		{"eslint --fix .", "other"},
		{"tsc --noEmit", "gate:typecheck:tsc"},
		{"tsc -b", "other"},
		{"prettier --check .", "gate:format-check:prettier"},
		{"prettier --write .", "other"},
		{"vitest", "gate:test:vitest"},
		{"vitest --watch", "other"},
		{"jest -w 4", "gate:test:jest"}, // -w is max workers, not watch
		{"ruff format --check .", "gate:format-check:ruff"},
		{"ruff check .", "gate:lint:ruff"},
		{"cargo clippy -- -D warnings", "gate:lint:cargo"},
		{"go test ./...", "gate:test:go"},
		{"bun test", "gate:test:bun"},
		{"tflint --only=terraform_unused_declarations", "gate:lint:tflint"}, // no deadcode slot until step 16
		{"./node_modules/.bin/eslint src", "gate:lint:eslint"},
		// Wrappers and tool runners unwrap to the tool.
		{"NODE_ENV=test vitest run", "gate:test:vitest"},
		{"cross-env CI=1 jest", "gate:test:jest"},
		{"pnpm exec eslint .", "gate:lint:eslint"},
		{"npx tsc --noEmit", "gate:typecheck:tsc"},
		{"bunx eslint .", "gate:lint:eslint"},
		{"uv run pytest", "gate:test:pytest"},
		{"bundle exec rubocop", "gate:lint:rubocop"},
		// References.
		{"npm run lint", "ref:package-scripts::lint"},
		{"npm t", "ref:package-scripts::test"},
		{"npm test -- --coverage", "ref:package-scripts::test:--coverage"},
		{"pnpm lint", "ref:package-scripts::lint"},
		{"pnpm eslint .", "gate:lint:eslint"}, // no eslint script: pnpm runs the binary
		{"yarn typecheck", "ref:package-scripts::typecheck"},
		{"make -C web lint", "ref:make:web:lint"},
		{"make lint", "ref:make::lint"},
		{"bundle exec rake spec", "ref:rake::spec"},
		{"poetry run poe test", "ref:poe::test"},
		{"run-s lint test", "ref:package-scripts::lint ref:package-scripts::test"},
		{`concurrently -n a,b "pnpm lint" "vitest run"`, "ref:package-scripts::lint gate:test:vitest"},
		// Fan-out across workspace packages.
		{"pnpm -r lint", "fanout"},
		{"pnpm --filter web lint", "fanout"},
		{"turbo run lint", "fanout"},
		// Sequences and context.
		{"cd web && tsc --noEmit", "cd gate:typecheck:tsc"},
		{"export CI=1; jest", "export gate:test:jest"},
		{"pnpm build && pnpm test", "other ref:package-scripts::test"},
		{"eslint . 2>&1", "gate:lint:eslint"},
		{"eslint . >/dev/null", "gate:lint:eslint"},
		// Unreadable with certainty: nothing counts.
		{"eslint . || true", "opaque:||"},
		{"eslint . | tee out", "opaque:|"},
		{"(cd web && eslint .)", "opaque:compound command"},
		{"eslint $(git diff --name-only)", "opaque:command substitution"},
		{"eslint . > report.txt", "opaque:redirect"},
		{"eslint $FILES", "other"},
		{"node scripts/check.js", "other"},
		{"if true; then eslint .; fi", "opaque:compound command"},
	} {
		if got := describe(Body(cfg, c.body, scripts)); got != c.want {
			t.Errorf("%q: %s, want %s", c.body, got, c.want)
		}
	}
}

// Every pattern kit.yml ships is detected from its minimal command, and
// turned away by its first forbidden flag or a missing required one.
func TestEveryToolPatternDetectsAndSkips(t *testing.T) {
	cfg := kitConfig(t)
	for _, check := range cfg.Checks {
		for _, p := range check.Tools {
			words := []string{p.Bin}
			if len(p.Sub) > 0 {
				words = append(words, p.Sub[0])
			}
			words = append(words, p.Require...)
			name := check.Name + "/" + strings.Join(words, " ")
			if got := describe(Body(cfg, strings.Join(words, " "), scripts)); !strings.HasPrefix(got, "gate:") {
				t.Errorf("%s: %s, want a gate", name, got)
				continue
			}
			if len(p.Forbid) > 0 {
				body := strings.Join(append(slices.Clone(words), p.Forbid[0]), " ")
				if got := describe(Body(cfg, body, scripts)); got == "gate:"+check.Name+":"+p.Bin {
					t.Errorf("%s with %s still counts as %s", name, p.Forbid[0], check.Name)
				}
			}
			if len(p.Require) > 0 {
				body := strings.Join(words[:len(words)-len(p.Require)], " ")
				if got := describe(Body(cfg, body, scripts)); got == "gate:"+check.Name+":"+p.Bin {
					t.Errorf("%s without %s still counts as %s", name, p.Require[0], check.Name)
				}
			}
		}
	}
}

func TestSingleGate(t *testing.T) {
	cfg := kitConfig(t)
	for body, want := range map[string]string{
		"eslint .":                       "lint",
		"cd web && tsc --noEmit":         "typecheck",
		"eslint . && prettier --check .": "",
		"pnpm build && vitest run":       "",
		"npm run lint":                   "",
	} {
		gate, ok := Body(cfg, body, scripts).SingleGate()
		if got := map[bool]string{true: gate.Slot}[ok]; got != want {
			t.Errorf("%q: single gate %q, want %q", body, got, want)
		}
	}
}
