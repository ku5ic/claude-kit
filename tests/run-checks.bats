#!/usr/bin/env bats
# Tests for ~/.dotfiles/claude-kit/bin/run-checks.sh.
#
# run-checks.sh runs a project's declared check scripts through its package
# manager; a check with no declared script is skipped, never synthesized by
# invoking a linter binary directly. These tests build fixture projects and
# assert on the emitted labels and exit code. stub_bin fakes tool binaries on
# PATH so tests do not depend on real toolchains being installed.
#
# Run with: bats tests/

setup() {
  RUN_CHECKS="$BATS_TEST_DIRNAME/../bin/run-checks.sh"
  PROJECT_DIR="$BATS_TEST_TMPDIR/project"
  STUB_DIR="$BATS_TEST_TMPDIR/stubs"
  mkdir -p "$PROJECT_DIR" "$STUB_DIR"
  PATH="$STUB_DIR:$PATH"
}

# stub_bin <name> <exit_code>
# Creates an executable at $STUB_DIR/<name> that ignores its arguments and
# exits with <exit_code>. $STUB_DIR is prepended onto PATH by setup().
stub_bin() {
  local name="$1" code="${2:-0}"
  cat >"$STUB_DIR/$name" <<EOF
#!/usr/bin/env bash
exit $code
EOF
  chmod +x "$STUB_DIR/$name"
}

run_checks() {
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

@test "js: no lint script skips lint" {
  printf '{}' >"$PROJECT_DIR/package.json"
  run run_checks
  [[ "$output" == *"SKIP js: lint (no lint script)"* ]]
}

@test "js: eslint config without a lint script is still skipped (no direct linter run)" {
  printf '{}' >"$PROJECT_DIR/package.json"
  printf 'export default [];\n' >"$PROJECT_DIR/eslint.config.js"
  run run_checks
  [[ "$output" == *"SKIP js: lint (no lint script)"* ]]
  [[ "$output" != *"eslint"* ]]
}

@test "js: typecheck script runs" {
  printf '{"scripts": {"typecheck": "tsc --noEmit"}}' >"$PROJECT_DIR/package.json"
  stub_bin npm 0
  run run_checks
  [[ "$output" == *"PASS ts: typecheck"* ]]
}

@test "js: type-check (hyphenated) script runs" {
  printf '{"scripts": {"type-check": "tsc --noEmit"}}' >"$PROJECT_DIR/package.json"
  stub_bin npm 0
  run run_checks
  [[ "$output" == *"PASS ts: typecheck"* ]]
}

@test "js: tsconfig without a typecheck script is skipped (no direct tsc run)" {
  printf '{}' >"$PROJECT_DIR/package.json"
  printf '{}' >"$PROJECT_DIR/tsconfig.json"
  run run_checks
  [[ "$output" == *"SKIP ts: typecheck (no typecheck script)"* ]]
}

@test "js: format:check script runs" {
  printf '{"scripts": {"format:check": "prettier --check ."}}' >"$PROJECT_DIR/package.json"
  stub_bin npm 0
  run run_checks
  [[ "$output" == *"PASS js: format-check"* ]]
}

@test "js: only a mutating format script skips format-check" {
  printf '{"scripts": {"format": "prettier --write ."}}' >"$PROJECT_DIR/package.json"
  run run_checks
  [[ "$output" == *"SKIP js: format-check (no format:check script; format would mutate)"* ]]
}

@test "js: test script runs via the package manager" {
  printf '{"scripts": {"test": "vitest run"}}' >"$PROJECT_DIR/package.json"
  stub_bin npm 0
  run run_checks
  [[ "$output" == *"PASS js: test"* ]]
}

@test "js: no test script skips test" {
  printf '{}' >"$PROJECT_DIR/package.json"
  run run_checks
  [[ "$output" == *"SKIP js: test (no test script)"* ]]
}

# Python: declared pdm/poe tasks run via the package manager.

@test "py: pdm script runs via pdm run" {
  printf '[tool.pdm.scripts]\nlint = "ruff check ."\n' >"$PROJECT_DIR/pyproject.toml"
  stub_bin pdm 0
  run run_checks
  [[ "$output" == *"PASS py: lint"* ]]
}

@test "py: poe task runs via the poe runner" {
  printf '[tool.poe.tasks]\ntest = "pytest"\n' >"$PROJECT_DIR/pyproject.toml"
  stub_bin poe 0
  run run_checks
  [[ "$output" == *"PASS py: test"* ]]
}

@test "py: Makefile lint target runs via make" {
  printf '[tool.ruff]\n' >"$PROJECT_DIR/pyproject.toml"
  printf 'lint:\n\truff check .\n' >"$PROJECT_DIR/Makefile"
  stub_bin make 0
  run run_checks
  [[ "$output" == *"PASS py: lint"* ]]
}

@test "py: Makefile typecheck and test targets run via make" {
  printf '[tool.pyright]\n' >"$PROJECT_DIR/pyproject.toml"
  printf 'typecheck:\n\tpyright apps/\ntest:\n\tpytest\n' >"$PROJECT_DIR/Makefile"
  stub_bin make 0
  run run_checks
  [[ "$output" == *"PASS py: typecheck"* ]]
  [[ "$output" == *"PASS py: test"* ]]
}

@test "py: Makefile without a matching target still skips that check" {
  printf '[tool.ruff]\n' >"$PROJECT_DIR/pyproject.toml"
  printf 'build:\n\techo build\n' >"$PROJECT_DIR/Makefile"
  run run_checks
  [[ "$output" == *"SKIP py: lint (no lint task)"* ]]
}

@test "py: requirements.txt plus a Makefile lint target runs via make" {
  printf 'django\n' >"$PROJECT_DIR/requirements.txt"
  printf 'lint:\n\truff check .\n' >"$PROJECT_DIR/Makefile"
  stub_bin make 0
  run run_checks
  [[ "$output" == *"PASS py: lint"* ]]
}

@test "py: no declared task skips the check (no direct tool run)" {
  printf '[tool.ruff]\n' >"$PROJECT_DIR/pyproject.toml"
  run run_checks
  [[ "$output" == *"SKIP py: lint (no lint task)"* ]]
  [[ "$output" != *"ruff"* ]]
}

@test "py: requirements.txt without pyproject skips all python checks" {
  printf 'requests\n' >"$PROJECT_DIR/requirements.txt"
  run run_checks
  [[ "$output" == *"SKIP py: lint (no lint task)"* ]]
  [[ "$output" == *"SKIP py: test (no test task)"* ]]
}

# Ruby: declared rake tasks run via bundler.

@test "rb: rake lint task runs via bundler" {
  printf "source 'https://rubygems.org'\n" >"$PROJECT_DIR/Gemfile"
  printf 'task :lint do\nend\n' >"$PROJECT_DIR/Rakefile"
  stub_bin bundle 0
  run run_checks
  [[ "$output" == *"PASS rb: lint"* ]]
}

@test "rb: rubocop config without a rake task is skipped (no direct rubocop run)" {
  printf "source 'https://rubygems.org'\n" >"$PROJECT_DIR/Gemfile"
  printf 'AllCops:\n' >"$PROJECT_DIR/.rubocop.yml"
  run run_checks
  [[ "$output" == *"SKIP rb: lint (no rake lint task)"* ]]
  [[ "$output" != *"rubocop"* ]]
}

# Go: the toolchain's own subcommands.

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

# Monorepo: manifests live one level deep under app subdirs.

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
  [[ "$output" == *"PASS py: lint [backend]"* ]]
}

@test "monorepo: frontend and backend both run in one invocation" {
  mkdir -p "$PROJECT_DIR/frontend" "$PROJECT_DIR/backend"
  printf '{"scripts": {"test": "vitest run"}}' >"$PROJECT_DIR/frontend/package.json"
  printf '[tool.pdm.scripts]\ntest = "pytest"\n' >"$PROJECT_DIR/backend/pyproject.toml"
  stub_bin npm 0
  stub_bin pdm 0
  run run_checks
  [[ "$output" == *"PASS js: test [frontend]"* ]]
  [[ "$output" == *"PASS py: test [backend]"* ]]
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

# summary line

@test "summary line reports pass/fail/skip counts" {
  printf '{}' >"$PROJECT_DIR/package.json"
  run run_checks
  [[ "$output" == *"checks:"*"passed"*"failed"*"skipped"* ]]
}
