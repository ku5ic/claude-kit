#!/usr/bin/env bats
# Tests for ~/.dotfiles/claude-kit/bin/run-checks.sh.
#
# run-checks.sh runs a project's declared check tasks, found through kit.yml's
# task_providers, in every subproject; a check with no declared task is
# skipped, never synthesized by invoking a linter binary directly. These tests
# build fixture repos and assert on the emitted labels and exit code. stub_bin
# fakes tool binaries on PATH so tests do not depend on real toolchains.
#
# Fixtures are git repos with every file added: subprojects come from tracked
# files. HOME is faked so the kit's caches and overlay stay in the tmpdir.
#
# Run with: bats tests/

setup() {
  RUN_CHECKS="$BATS_TEST_DIRNAME/../bin/run-checks.sh"
  PROJECT_DIR="$BATS_TEST_TMPDIR/project"
  STUB_DIR="$BATS_TEST_TMPDIR/stubs"
  mkdir -p "$PROJECT_DIR" "$STUB_DIR"
  git init -q -b main "$PROJECT_DIR"
  PATH="$STUB_DIR:$PATH"
  export HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$HOME/.claude"
  unset CLAUDE_PLUGIN_ROOT CLAUDE_CONFIG_DIR
}

# stub_bin <name> <exit_code>
# Creates an executable at $STUB_DIR/<name> that records its arguments to
# $STUB_DIR/<name>.calls and exits with <exit_code>.
stub_bin() {
  local name="$1" code="${2:-0}"
  cat >"$STUB_DIR/$name" <<EOF
#!/usr/bin/env bash
echo "\$PWD \$*" >>"$STUB_DIR/$name.calls"
exit $code
EOF
  chmod +x "$STUB_DIR/$name"
}

run_checks() {
  git -C "$PROJECT_DIR" add -A
  (cd "$PROJECT_DIR" && "$RUN_CHECKS")
}

# JS/TS: declared package.json scripts run via the package manager.

@test "js: lint script runs via the package manager" {
  printf '{"scripts": {"lint": "eslint ."}}' >"$PROJECT_DIR/package.json"
  stub_bin npm 0
  run run_checks
  [[ "$output" == *"PASS js: lint (lint)"* ]]
}

@test "js: lint script failure is reflected in exit code" {
  printf '{"scripts": {"lint": "eslint ."}}' >"$PROJECT_DIR/package.json"
  stub_bin npm 1
  run run_checks
  [[ "$output" == *"FAIL js: lint (lint)"* ]]
  [ "$status" -ge 1 ]
}

@test "js: every lint-like script runs, :fix variants never do" {
  printf '{"scripts": {"lint": "eslint .", "stylelint": "stylelint", "lint:css": "x", "lint:fix": "eslint --fix"}}' >"$PROJECT_DIR/package.json"
  stub_bin npm 0
  run run_checks
  [[ "$output" == *"PASS js: lint (lint)"* ]]
  [[ "$output" == *"PASS js: lint (stylelint)"* ]]
  [[ "$output" == *"PASS js: lint (lint:css)"* ]]
  [[ "$output" != *"lint:fix"* ]]
}

@test "js: no lint script skips lint" {
  printf '{}' >"$PROJECT_DIR/package.json"
  run run_checks
  [[ "$output" == *"SKIP js: lint (no lint task)"* ]]
}

@test "js: eslint config without a lint script is still skipped (no direct linter run)" {
  printf '{}' >"$PROJECT_DIR/package.json"
  printf 'export default [];\n' >"$PROJECT_DIR/eslint.config.js"
  run run_checks
  [[ "$output" == *"SKIP js: lint (no lint task)"* ]]
  [[ "$output" != *"eslint"* ]]
}

@test "js: typecheck script runs" {
  printf '{"scripts": {"typecheck": "tsc --noEmit"}}' >"$PROJECT_DIR/package.json"
  stub_bin npm 0
  run run_checks
  [[ "$output" == *"PASS js: typecheck (typecheck)"* ]]
}

@test "js: type-check (hyphenated) script runs" {
  printf '{"scripts": {"type-check": "tsc --noEmit"}}' >"$PROJECT_DIR/package.json"
  stub_bin npm 0
  run run_checks
  [[ "$output" == *"PASS js: typecheck (type-check)"* ]]
}

@test "js: tsconfig without a typecheck script is skipped (no direct tsc run)" {
  printf '{}' >"$PROJECT_DIR/package.json"
  printf '{}' >"$PROJECT_DIR/tsconfig.json"
  run run_checks
  [[ "$output" == *"SKIP js: typecheck (no typecheck task)"* ]]
}

@test "js: format:check script runs" {
  printf '{"scripts": {"format:check": "prettier --check ."}}' >"$PROJECT_DIR/package.json"
  stub_bin npm 0
  run run_checks
  [[ "$output" == *"PASS js: format-check (format:check)"* ]]
}

@test "js: only a mutating format script skips format-check" {
  printf '{"scripts": {"format": "prettier --write ."}}' >"$PROJECT_DIR/package.json"
  stub_bin npm 0
  run run_checks
  [[ "$output" == *"SKIP js: format-check (no format-check task)"* ]]
  [ ! -e "$STUB_DIR/npm.calls" ]
}

@test "js: test script runs via the lockfile's package manager" {
  printf '{"scripts": {"test": "vitest run"}}' >"$PROJECT_DIR/package.json"
  touch "$PROJECT_DIR/pnpm-lock.yaml"
  stub_bin pnpm 0
  run run_checks
  [[ "$output" == *"PASS js: test (test)"* ]]
  [[ "$(cat "$STUB_DIR/pnpm.calls")" == *"run test" ]]
}

@test "js: no test script skips test" {
  printf '{}' >"$PROJECT_DIR/package.json"
  run run_checks
  [[ "$output" == *"SKIP js: test (no test task)"* ]]
}

# Python: declared pdm/poe tasks and Makefile targets.

@test "py: pdm script runs via pdm run" {
  printf '[tool.pdm.scripts]\nlint = "ruff check ."\n' >"$PROJECT_DIR/pyproject.toml"
  stub_bin pdm 0
  run run_checks
  [[ "$output" == *"PASS python: lint (lint)"* ]]
}

@test "py: poe task runs via the poe runner" {
  printf '[tool.poe.tasks]\ntest = "pytest"\n' >"$PROJECT_DIR/pyproject.toml"
  stub_bin poe 0
  run run_checks
  [[ "$output" == *"PASS python: test (test)"* ]]
}

@test "py: a poetry service under a pnpm root runs poe through poetry" {
  printf '{"name":"root","private":true}\n' >"$PROJECT_DIR/package.json"
  touch "$PROJECT_DIR/pnpm-lock.yaml"
  mkdir -p "$PROJECT_DIR/svc"
  printf '[tool.poe.tasks]\nlint = "ruff check"\n' >"$PROJECT_DIR/svc/pyproject.toml"
  touch "$PROJECT_DIR/svc/poetry.lock"
  stub_bin poetry 0
  run run_checks
  [[ "$output" == *"PASS python: lint (lint) [svc]"* ]]
  [[ "$(cat "$STUB_DIR/poetry.calls")" == *"run poe lint" ]]
}

@test "js: a subproject's own lockfile beats the root's" {
  printf '{"name":"root","private":true}\n' >"$PROJECT_DIR/package.json"
  touch "$PROJECT_DIR/pnpm-lock.yaml"
  mkdir -p "$PROJECT_DIR/web"
  printf '{"name":"web","scripts":{"test":"vitest"}}\n' >"$PROJECT_DIR/web/package.json"
  touch "$PROJECT_DIR/web/yarn.lock"
  stub_bin yarn 0
  run run_checks
  [[ "$output" == *"PASS js: test (test) [web]"* ]]
  [[ "$(cat "$STUB_DIR/yarn.calls")" == *"run test" ]]
}

@test "py: poe task runs through poetry in a poetry project" {
  printf '[tool.poe.tasks]\ntest = "pytest"\n' >"$PROJECT_DIR/pyproject.toml"
  touch "$PROJECT_DIR/poetry.lock"
  stub_bin poetry 0
  run run_checks
  [[ "$output" == *"PASS python: test (test)"* ]]
  [[ "$(cat "$STUB_DIR/poetry.calls")" == *"run poe test" ]]
}

@test "py: Makefile lint target runs via make" {
  printf '[tool.ruff]\n' >"$PROJECT_DIR/pyproject.toml"
  printf 'lint:\n\truff check .\n' >"$PROJECT_DIR/Makefile"
  stub_bin make 0
  run run_checks
  [[ "$output" == *"PASS make: lint (lint)"* ]]
}

@test "py: Makefile typecheck and test targets run via make" {
  printf '[tool.pyright]\n' >"$PROJECT_DIR/pyproject.toml"
  printf 'typecheck:\n\tpyright apps/\ntest:\n\tpytest\n' >"$PROJECT_DIR/Makefile"
  stub_bin make 0
  run run_checks
  [[ "$output" == *"PASS make: typecheck (typecheck)"* ]]
  [[ "$output" == *"PASS make: test (test)"* ]]
}

@test "py: Makefile without a matching target still skips that check" {
  printf '[tool.ruff]\n' >"$PROJECT_DIR/pyproject.toml"
  printf 'build:\n\techo build\n' >"$PROJECT_DIR/Makefile"
  run run_checks
  [[ "$output" == *"SKIP python: lint (no lint task)"* ]]
}

@test "py: requirements.txt plus a Makefile lint target runs via make" {
  printf 'django\n' >"$PROJECT_DIR/requirements.txt"
  printf 'lint:\n\truff check .\n' >"$PROJECT_DIR/Makefile"
  stub_bin make 0
  run run_checks
  [[ "$output" == *"PASS make: lint (lint)"* ]]
}

@test "py: no declared task skips the check (no direct tool run)" {
  printf '[tool.ruff]\n' >"$PROJECT_DIR/pyproject.toml"
  run run_checks
  [[ "$output" == *"SKIP python: lint (no lint task)"* ]]
  [[ "$output" != *"ruff"* ]]
}

@test "py: requirements.txt alone declares no tasks, so nothing runs" {
  printf 'requests\n' >"$PROJECT_DIR/requirements.txt"
  run run_checks
  [ "$status" -eq 0 ]
  [[ "$output" == *"checks: 0 passed, 0 failed, 0 skipped"* ]]
}

# Ruby: declared rake tasks run via bundler.

@test "rb: rake lint task runs via bundler" {
  printf "source 'https://rubygems.org'\n" >"$PROJECT_DIR/Gemfile"
  printf 'task :lint do\nend\n' >"$PROJECT_DIR/Rakefile"
  stub_bin bundle 0
  run run_checks
  [[ "$output" == *"PASS ruby: lint (lint)"* ]]
}

@test "rb: rubocop config without a Rakefile runs nothing (no direct rubocop run)" {
  printf "source 'https://rubygems.org'\n" >"$PROJECT_DIR/Gemfile"
  printf 'AllCops:\n' >"$PROJECT_DIR/.rubocop.yml"
  run run_checks
  [[ "$output" != *"rubocop"* ]]
  [[ "$output" == *"checks: 0 passed"* ]]
}

# Toolchain checks: the stack's own subcommands.

@test "go: vet and test run" {
  printf 'module example.com/fixture\n\ngo 1.22\n' >"$PROJECT_DIR/go.mod"
  stub_bin go 0
  run run_checks
  [[ "$output" == *"PASS go: vet"* ]]
  [[ "$output" == *"PASS go: test"* ]]
}

@test "go: vet failure is reported and reflected in exit code" {
  printf 'module example.com/fixture\n\ngo 1.22\n' >"$PROJECT_DIR/go.mod"
  stub_bin go 1
  run run_checks
  [[ "$output" == *"FAIL go: vet"* ]]
  [ "$status" -ge 1 ]
}

@test "rust: cargo checks run where Cargo.toml is" {
  printf '[package]\nname = "x"\n' >"$PROJECT_DIR/Cargo.toml"
  stub_bin cargo 0
  run run_checks
  [[ "$output" == *"PASS rust: check"* ]]
  [[ "$output" == *"PASS rust: clippy"* ]]
  [[ "$output" == *"PASS rust: fmt"* ]]
  [[ "$output" == *"PASS rust: test"* ]]
}

@test "opentofu: fmt runs, validate skips until init made .terraform/" {
  touch "$PROJECT_DIR/.terraform.lock.hcl"
  stub_bin tofu 0
  run run_checks
  [[ "$output" == *"PASS opentofu: fmt"* ]]
  [[ "$output" == *"SKIP opentofu: validate (no .terraform/ yet)"* ]]
  grep -q " fmt -check -recursive$" "$STUB_DIR/tofu.calls"

  mkdir "$PROJECT_DIR/.terraform"
  run run_checks
  [[ "$output" == *"PASS opentofu: validate"* ]]
}

# Monorepo: every subproject, at any depth.

@test "monorepo: subdir package.json is discovered and labeled" {
  mkdir -p "$PROJECT_DIR/frontend"
  printf '{"scripts": {"lint": "eslint ."}}' >"$PROJECT_DIR/frontend/package.json"
  stub_bin npm 0
  run run_checks
  [[ "$output" == *"PASS js: lint (lint) [frontend]"* ]]
}

@test "monorepo: subdir pyproject task is discovered and labeled" {
  mkdir -p "$PROJECT_DIR/backend"
  printf '[tool.pdm.scripts]\nlint = "ruff check ."\n' >"$PROJECT_DIR/backend/pyproject.toml"
  stub_bin pdm 0
  run run_checks
  [[ "$output" == *"PASS python: lint (lint) [backend]"* ]]
}

@test "monorepo: frontend and backend both run in one invocation" {
  mkdir -p "$PROJECT_DIR/frontend" "$PROJECT_DIR/backend"
  printf '{"scripts": {"test": "vitest run"}}' >"$PROJECT_DIR/frontend/package.json"
  printf '[tool.pdm.scripts]\ntest = "pytest"\n' >"$PROJECT_DIR/backend/pyproject.toml"
  stub_bin npm 0
  stub_bin pdm 0
  run run_checks
  [[ "$output" == *"PASS js: test (test) [frontend]"* ]]
  [[ "$output" == *"PASS python: test (test) [backend]"* ]]
}

@test "monorepo: a failing subdir check drives the overall exit code" {
  mkdir -p "$PROJECT_DIR/frontend"
  printf '{"scripts": {"lint": "eslint ."}}' >"$PROJECT_DIR/frontend/package.json"
  stub_bin npm 1
  run run_checks
  [[ "$output" == *"FAIL js: lint (lint) [frontend]"* ]]
  [ "$status" -ge 1 ]
}

@test "monorepo: root and subdir manifests both run" {
  printf '{"scripts": {"lint": "eslint ."}}' >"$PROJECT_DIR/package.json"
  mkdir -p "$PROJECT_DIR/frontend"
  printf '{"scripts": {"lint": "eslint ."}}' >"$PROJECT_DIR/frontend/package.json"
  stub_bin npm 0
  run run_checks
  [[ "$output" == *"PASS js: lint (lint)"* ]]
  [[ "$output" == *"PASS js: lint (lint) [frontend]"* ]]
}

@test "monorepo: a nested pnpm workspace package's test runs, in its own dir" {
  printf '{"name":"root","private":true}\n' >"$PROJECT_DIR/package.json"
  printf 'packages:\n  - "packages/*"\n' >"$PROJECT_DIR/pnpm-workspace.yaml"
  touch "$PROJECT_DIR/pnpm-lock.yaml"
  mkdir -p "$PROJECT_DIR/packages/a"
  printf '{"name":"a","scripts":{"test":"vitest run"}}\n' >"$PROJECT_DIR/packages/a/package.json"
  stub_bin pnpm 0
  run run_checks
  [[ "$output" == *"PASS js: test (test) [packages/a]"* ]]
  [[ "$(cat "$STUB_DIR/pnpm.calls")" == *"/packages/a run test" ]]
}

@test "--only checks just the named subprojects" {
  printf '{"scripts":{"lint":"eslint ."}}\n' >"$PROJECT_DIR/package.json"
  mkdir -p "$PROJECT_DIR/packages/a" "$PROJECT_DIR/services/api"
  printf '{"scripts":{"test":"vitest"}}\n' >"$PROJECT_DIR/packages/a/package.json"
  printf '[tool.pdm.scripts]\ntest = "pytest"\n' >"$PROJECT_DIR/services/api/pyproject.toml"
  stub_bin npm 0
  stub_bin pdm 0
  git -C "$PROJECT_DIR" add -A
  run bash -c "cd '$PROJECT_DIR' && '$RUN_CHECKS' --only services/api"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS python: test (test) [services/api]"* ]]
  [[ "$output" != *"packages/a"* ]]
  [[ "$output" != *"js: lint"* ]]
}

# Orchestrators: turbo or nx run JS checks once, for affected packages.

# pnpm workspace with turbo.json declaring test and lint, a Python service,
# and a recording orchestrator binary <name> in node_modules/.bin.
make_orchestrated() {
  local name="$1" config="$2"
  printf '{"name":"root","private":true,"scripts":{"test":"turbo run test"}}\n' >"$PROJECT_DIR/package.json"
  printf 'packages:\n  - "packages/*"\n' >"$PROJECT_DIR/pnpm-workspace.yaml"
  touch "$PROJECT_DIR/pnpm-lock.yaml"
  printf '%s\n' "$config" >"$PROJECT_DIR/$name.json"
  mkdir -p "$PROJECT_DIR/packages/a" "$PROJECT_DIR/services/api" "$PROJECT_DIR/node_modules/.bin"
  printf '{"name":"a","scripts":{"test":"vitest","lint":"eslint ."}}\n' >"$PROJECT_DIR/packages/a/package.json"
  printf '[tool.pdm.scripts]\ntest = "pytest"\n' >"$PROJECT_DIR/services/api/pyproject.toml"
  printf '#!/usr/bin/env bash\necho "$*" >>"%s"\n' "$STUB_DIR/$name.calls" >"$PROJECT_DIR/node_modules/.bin/$name"
  chmod +x "$PROJECT_DIR/node_modules/.bin/$name"
  printf 'node_modules\n' >"$PROJECT_DIR/.gitignore"
  stub_bin pnpm 0
  stub_bin pdm 0
}

run_checks_only() {
  git -C "$PROJECT_DIR" add -A
  (cd "$PROJECT_DIR" && "$RUN_CHECKS" --only "$@")
}

@test "turbo: an edit in packages/a runs turbo once per check, no per-package task" {
  make_orchestrated turbo '{"tasks":{"test":{},"lint":{},"build":{}}}'
  run run_checks_only packages/a
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS js: test (turbo affected: test)"* ]]
  [[ "$output" == *"PASS js: lint (turbo affected: lint)"* ]]
  [ "$(cat "$STUB_DIR/turbo.calls")" = "run lint --filter=...[HEAD]
run test --filter=...[HEAD]" ]
  [ ! -e "$STUB_DIR/pnpm.calls" ]
  [[ "$output" != *"(test) [packages/a]"* ]]
  [[ "$output" != *"(lint) [packages/a]"* ]]
}

@test "turbo: a 1.x pipeline key is read too" {
  make_orchestrated turbo '{"pipeline":{"test":{}}}'
  run run_checks_only packages/a
  [[ "$output" == *"PASS js: test (turbo affected: test)"* ]]
}

@test "turbo: checks turbo declares no task for still run per package" {
  make_orchestrated turbo '{"tasks":{"test":{}}}'
  run run_checks_only packages/a
  [[ "$output" == *"PASS js: test (turbo affected: test)"* ]]
  [[ "$output" == *"PASS js: lint (lint) [packages/a]"* ]]
}

@test "turbo: a Python-only scope never runs turbo" {
  make_orchestrated turbo '{"tasks":{"test":{}}}'
  run run_checks_only services/api
  [[ "$output" == *"PASS python: test (test) [services/api]"* ]]
  [ ! -e "$STUB_DIR/turbo.calls" ]
}

@test "turbo: without its binary in node_modules/.bin, packages run their own tasks" {
  make_orchestrated turbo '{"tasks":{"test":{}}}'
  rm "$PROJECT_DIR/node_modules/.bin/turbo"
  run run_checks_only packages/a
  [[ "$output" == *"PASS js: test (test) [packages/a]"* ]]
  [[ "$output" != *"turbo affected"* ]]
}

@test "nx: targetDefaults run through nx affected --uncommitted" {
  make_orchestrated nx '{"targetDefaults":{"test":{},"typecheck":{}}}'
  run run_checks_only packages/a
  [[ "$output" == *"PASS js: typecheck (nx affected: typecheck)"* ]]
  [[ "$output" == *"PASS js: test (nx affected: test)"* ]]
  [ "$(cat "$STUB_DIR/nx.calls")" = "affected -t typecheck --uncommitted
affected -t test --uncommitted" ]
}

# kit.yml overlay: a new provider is data, not code.

@test "an overlay-defined composer provider runs its test script" {
  cat >"$HOME/.claude/claude-kit.local.yml" <<'YAML'
task_providers:
  - name: composer
    stack: php
    manifests: [composer.json]
    extractor: json_keys
    arg: .scripts
    run: "composer run {task}"
YAML
  printf '{"scripts": {"test": "phpunit"}}' >"$PROJECT_DIR/composer.json"
  stub_bin composer 0
  run run_checks
  [[ "$output" == *"PASS php: test (test)"* ]]
  [[ "$(cat "$STUB_DIR/composer.calls")" == *"run test" ]]
}

# summary line

@test "a relative script path from a subdirectory still finds the lib" {
  printf 'module example.com/fixture\n\ngo 1.22\n' >"$PROJECT_DIR/go.mod"
  mkdir -p "$PROJECT_DIR/sub"
  stub_bin go 0
  git -C "$PROJECT_DIR" add -A
  # ../ up to /, then the script's absolute path: relative from sub/.
  local sub up="" script
  sub="$(cd "$PROJECT_DIR/sub" && pwd -P)"
  script="$(cd "$BATS_TEST_DIRNAME/../bin" && pwd)/run-checks.sh"
  up="$(printf '%s' "${sub//[!\/]/}" | sed 's|/|../|g')"
  cd "$sub"
  run "$up${script#/}"
  [[ "$output" == *"PASS go: vet"* ]]
}

@test "summary line reports pass/fail/skip counts" {
  printf '{}' >"$PROJECT_DIR/package.json"
  run run_checks
  [[ "$output" == *"checks:"*"passed"*"failed"*"skipped"* ]]
}
