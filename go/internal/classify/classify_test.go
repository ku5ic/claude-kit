package classify

import (
	"slices"
	"strings"
	"testing"
)

// scripts is the package.json the lookup answers for: lint, test, typecheck.
func scripts(provider, dir, name string) bool {
	if provider == "just" {
		return name == "lint" || name == "test"
	}
	return provider == "package-scripts" && dir == "" && (name == "lint" || name == "test" || name == "typecheck")
}

// describe renders a result compactly: "ref:make:web:lint", "fanout",
// "cd", "export", "other", or "opaque:<why>".
func describe(r Result) string {
	if r.Opaque != "" {
		return "opaque:" + r.Opaque
	}
	var parts []string
	for _, c := range r.Commands {
		switch c.Kind {
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
	for _, c := range []struct{ body, want string }{
		// Tools are other commands: gap-fill answers what they are.
		{"eslint .", "other"},
		{"find scripts -name '*.sh' -exec shellcheck -S warning {} +", "other"},
		{"NODE_ENV=test vitest run", "other"},
		// References.
		{"npm run lint", "ref:package-scripts::lint"},
		{"npm t", "ref:package-scripts::test"},
		{"npm test -- --coverage", "ref:package-scripts::test:--coverage"},
		{"pnpm lint", "ref:package-scripts::lint"},
		{"pnpm eslint .", "other"}, // no eslint script: pnpm runs the binary
		{"yarn typecheck", "ref:package-scripts::typecheck"},
		{"make -C web lint", "ref:make:web:lint"},
		{"make lint", "ref:make::lint"},
		{"make GOARCH=arm64 build", "ref:make::build"},
		{"make -j 4 test", "ref:make::test"},
		{"make -j lint", "ref:make::lint"},
		{"make lint unit", "ref:make::lint ref:make::unit"},
		{"make lint -C tools unit", "ref:make:tools:lint ref:make:tools:unit"},
		{"make -C /opt/app lint", "other"},
		{"make -C ~/app lint", "other"},
		{"just lint test", "ref:just::lint ref:just::test"},
		{"just deploy prod", "ref:just::deploy:prod"},
		{"make -f Makefile.ci lint", "other"},
		{"just os=linux build", "ref:just::build"},
		{"just --justfile ci.just lint", "other"},
		{"bundle exec rake spec", "ref:rake::spec"},
		{"poetry run poe test", "ref:poe::test"},
		{"run-s lint test", "ref:package-scripts::lint ref:package-scripts::test"},
		{`concurrently -n a,b "pnpm lint" "vitest run"`, "ref:package-scripts::lint other"},
		// Fan-out across workspace packages.
		{"pnpm -r lint", "fanout"},
		{"pnpm --filter web lint", "fanout"},
		{"turbo run lint", "fanout"},
		// Sequences and context.
		{`cd "$APP_DIR" && eslint .`, "other other"},
		{"cd web && tsc --noEmit", "cd other"},
		{"export CI=1; jest", "export other"},
		{"pnpm build && pnpm test", "other ref:package-scripts::test"},
		{"eslint . 2>&1", "other"},
		{"eslint . >/dev/null", "other"},
		// Unreadable with certainty.
		{"eslint . || true", "opaque:||"},
		{"eslint . | tee out", "opaque:|"},
		{"(cd web && eslint .)", "opaque:compound command"},
		{"eslint $(git diff --name-only)", "opaque:command substitution"},
		{"eslint . > report.txt", "opaque:redirect"},
		{"if true; then eslint .; fi", "opaque:compound command"},
	} {
		if got := describe(Body(c.body, scripts)); got != c.want {
			t.Errorf("%q: %s, want %s", c.body, got, c.want)
		}
	}
}

func TestBodyDetails(t *testing.T) {
	r := Body("export PATH\nexport CI=1 NODE_ENV\ncd web && eslint 'a*' src/*.ts", scripts)
	if len(r.Commands) != 4 {
		t.Fatalf("commands: %+v", r.Commands)
	}
	// A bare name re-exports what's already set; it sets nothing.
	if env := r.Commands[0].Env; len(env) != 0 {
		t.Errorf("export PATH: env %q, want none", env)
	}
	if env := r.Commands[1].Env; !slices.Equal(env, []string{"CI=1"}) {
		t.Errorf("export CI=1 NODE_ENV: env %q, want [CI=1]", env)
	}
	if lines := []uint{r.Commands[0].Line, r.Commands[1].Line, r.Commands[2].Line, r.Commands[3].Line}; !slices.Equal(lines, []uint{1, 2, 3, 3}) {
		t.Errorf("lines %v, want [1 2 3 3]", lines)
	}
	if globs := r.Commands[3].Globs; !slices.Equal(globs, []string{"src/*.ts"}) {
		t.Errorf("globs %q, want [src/*.ts]", globs)
	}
}
