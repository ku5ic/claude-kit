package classify

import (
	"slices"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// scripts is the package.json the lookup answers for: lint, test, typecheck.
func scripts(provider, dir, name string) bool {
	if provider == "just" {
		return name == "lint" || name == "test"
	}
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
	cfg := testutil.KitConfig(t)
	for _, c := range []struct{ body, want string }{
		// Plain tools, by each slot's patterns.
		{"eslint .", "gate:lint:eslint"},
		{"eslint --fix .", "other"},
		{"tsc --noEmit", "gate:typecheck:tsc"},
		{"tsc -b", "other"},
		{"tsc --noEmit=false", "other"}, // a required flag set off isn't set
		{"prettier --check .", "gate:format-check:prettier"},
		{"prettier --write .", "other"},
		{"vitest", "gate:test:vitest"},
		{"vitest --watch", "other"},
		{"vitest --watch=false", "gate:test:vitest"}, // set off, not on
		{"jest --watchAll=false", "gate:test:jest"},
		{"jest -w 4", "gate:test:jest"}, // -w is max workers, not watch
		{"ruff format --check .", "gate:format-check:ruff"},
		{"gofmt -l .", "other"}, // exits 0 whatever it lists
		{"shfmt -d .", "gate:format-check:shfmt"},
		{"ruff check .", "gate:lint:ruff"},
		{"cargo clippy -- -D warnings", "gate:lint:cargo"},
		{"go test ./...", "gate:test:go"},
		{"bun test", "gate:test:bun"},
		{"tflint --only=terraform_unused_declarations", "gate:deadcode:tflint"}, // the most required flags wins
		{"tflint", "gate:lint:tflint"},
		{"knip --reporter compact", "gate:deadcode:knip"},
		{"cargo machete", "gate:deadcode:cargo"},
		{"./node_modules/.bin/eslint src", "gate:lint:eslint"},
		// Wrappers and tool runners unwrap to the tool.
		{"NODE_ENV=test vitest run", "gate:test:vitest"},
		{"cross-env CI=1 jest", "gate:test:jest"},
		{"pnpm exec eslint .", "gate:lint:eslint"},
		{"npx tsc --noEmit", "gate:typecheck:tsc"},
		{"bunx eslint .", "gate:lint:eslint"},
		{"uv run pytest", "gate:test:pytest"},
		{"bundle exec rubocop", "gate:lint:rubocop"},
		{"go tool deadcode -test ./...", "gate:deadcode:deadcode"},
		{"rubocop -x", "other"},
		{"rubocop --auto-correct", "other"},
		{"rubocop --disable-uncorrectable", "other"},
		// References.
		{"npm run lint", "ref:package-scripts::lint"},
		{"npm t", "ref:package-scripts::test"},
		{"npm test -- --coverage", "ref:package-scripts::test:--coverage"},
		{"pnpm lint", "ref:package-scripts::lint"},
		{"pnpm eslint .", "gate:lint:eslint"}, // no eslint script: pnpm runs the binary
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
		{`cd "$APP_DIR" && eslint .`, "other gate:lint:eslint"},
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
		{"shellcheck scripts/*.sh", "gate:lint:shellcheck"},
		{"shellcheck ~/a.sh", "other"},
		{"shellcheck {a,b}.sh", "other"},
		{"shellcheck '{a,b}.sh'", "gate:lint:shellcheck"},
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
	cfg := testutil.KitConfig(t)
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

func TestBodyDetails(t *testing.T) {
	cfg := testutil.KitConfig(t)
	r := Body(cfg, "export PATH\nexport CI=1 NODE_ENV\ncd web && eslint 'a*' src/*.ts", scripts)
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

func TestSingleGate(t *testing.T) {
	cfg := testutil.KitConfig(t)
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
