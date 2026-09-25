#!/usr/bin/env bash
# Runs a project's declared check scripts through its package manager. Scans the
# repo root and each immediate subdirectory (one level deep), so monorepos that
# keep package.json/pyproject.toml/etc. under app subdirs (frontend/, backend/)
# are covered, not only single-package repos rooted at the git top level.
# Discovery is script-based: run-checks.sh runs the scripts/tasks a project
# declares (package.json scripts, Makefile targets, pdm/poe tasks, rake tasks)
# via the package manager, plus the build tool's own check subcommands
# (cargo, go). It never
# invokes a third-party linter binary directly off a config file. A check with
# no declared script is skipped, not synthesized. Subdirectory checks are
# labeled with the subdir name, e.g. "PASS js: lint [frontend]".
# Each section is independent: failures are reported, not aborted.
set -uo pipefail
shopt -s nullglob

root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$root" || exit 1

# shellcheck source=_lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"

pass=0
fail=0
skip=0

run() {
  local label="$1"
  shift
  local out
  # Portable template: `mktemp -t <prefix>` differs between BSD (macOS) and GNU
  # (Linux CI); an explicit path template with X's behaves the same on both.
  out="$(mktemp "${TMPDIR:-/tmp}/run-checks.XXXXXX")"
  if "$@" >"$out" 2>&1; then
    echo "PASS $label"
    pass=$((pass + 1))
  else
    echo "FAIL $label ($*)"
    head -30 "$out"
    fail=$((fail + 1))
  fi
  rm -f "$out"
}

skip_msg() {
  echo "SKIP $1"
  skip=$((skip + 1))
}

# True when a Makefile in the current dir declares a <target>. Checks the three
# filenames GNU make looks for, in its precedence order.
make_has_target() {
  local target="$1" mf
  for mf in GNUmakefile makefile Makefile; do
    [[ -f "$mf" ]] && grep -qE "^${target}:" "$mf" && return 0
  done
  return 1
}

# Prints the command that runs a declared Python task <name>, or returns 1 when
# no supported task runner declares it. Python has no single scripts table, so
# the common task runners are supported, in this precedence:
#   Makefile <name> target    -> make <name>
#   [tool.pdm.scripts.<name>] -> pdm run <name>
#   [tool.poe.tasks.<name>]   -> poe <name> (through poetry when available)
py_task_cmd() {
  local name="$1"
  if make_has_target "$name"; then
    printf 'make %s' "$name"
    return 0
  fi
  [[ -f pyproject.toml ]] || return 1
  if yq -p toml -e ".tool.pdm.scripts.\"$name\"" pyproject.toml >/dev/null 2>&1; then
    printf 'pdm run %s' "$name"
    return 0
  fi
  if yq -p toml -e ".tool.poe.tasks.\"$name\"" pyproject.toml >/dev/null 2>&1; then
    if command -v poetry >/dev/null 2>&1; then
      printf 'poetry run poe %s' "$name"
    else
      printf 'poe %s' "$name"
    fi
    return 0
  fi
  return 1
}

# Runs the declared Python task <name> under <label> via the package manager,
# or skips when no supported script table declares it.
run_py_task() {
  local name="$1" label="$2" cmd
  if cmd="$(py_task_cmd "$name")"; then
    local -a parts
    read -ra parts <<<"$cmd"
    run "$label" "${parts[@]}"
  else
    skip_msg "$label (no $name task)"
  fi
}

# Runs every language's checks in <dir>, appending <sfx> to each label so a
# monorepo's per-package results are distinguishable. Changes the working
# directory to <dir> (absolute) so relative manifest paths and tool invocations
# resolve there; counters stay in the caller because this runs in the main shell.
check_dir() {
  local dir="$1" sfx="$2"
  cd "$dir" || return

  # JS/TS: run declared package.json scripts through the package manager.
  if [[ -f package.json ]]; then
    local pm
    pm="$(resolve_package_manager ".")"
    pm="${pm:-npm}"

    if jq -e '.scripts.typecheck' package.json >/dev/null 2>&1; then
      run "ts: typecheck$sfx" "$pm" run typecheck
    elif jq -e '.scripts["type-check"]' package.json >/dev/null 2>&1; then
      run "ts: typecheck$sfx" "$pm" run type-check
    else
      skip_msg "ts: typecheck$sfx (no typecheck script)"
    fi

    # Projects commonly split linting across multiple scripts (eslint,
    # stylelint) instead of a single `lint` alias. Run every declared
    # lint-like script, not just the first match; skip `:fix` variants since
    # those mutate files rather than check them.
    local -a lint_scripts=()
    while IFS= read -r script; do
      lint_scripts+=("$script")
    done < <(jq -r '.scripts | keys[] | select(test("^(lint|eslint|stylelint)(:|$)") and (test(":fix$") | not))' package.json)

    if [[ ${#lint_scripts[@]} -gt 0 ]]; then
      for script in "${lint_scripts[@]}"; do
        run "js: lint ($script)$sfx" "$pm" run "$script"
      done
    else
      skip_msg "js: lint$sfx (no lint script)"
    fi

    if jq -e '.scripts["format:check"]' package.json >/dev/null 2>&1; then
      run "js: format-check$sfx" "$pm" run format:check
    elif jq -e '.scripts.format' package.json >/dev/null 2>&1; then
      skip_msg "js: format-check$sfx (no format:check script; format would mutate)"
    else
      skip_msg "js: format-check$sfx (no format:check script)"
    fi

    if jq -e '.scripts.test' package.json >/dev/null 2>&1; then
      run "js: test$sfx" "$pm" run test
    else
      skip_msg "js: test$sfx (no test script)"
    fi
  fi

  # Python: run declared pdm/poe tasks through the package manager.
  if [[ -f pyproject.toml || -f requirements.txt ]]; then
    run_py_task lint "py: lint$sfx"
    run_py_task typecheck "py: typecheck$sfx"
    run_py_task test "py: test$sfx"
  fi

  # Ruby: run declared rake tasks through bundler.
  if [[ -f Gemfile ]]; then
    if [[ -f Rakefile ]] && grep -q "task.*:lint" Rakefile 2>/dev/null; then
      run "rb: lint$sfx" bundle exec rake lint
    else
      skip_msg "rb: lint$sfx (no rake lint task)"
    fi
    if [[ -f Rakefile ]] && grep -q "task.*:test\|RSpec" Rakefile 2>/dev/null; then
      run "rb: test$sfx" bundle exec rake test
    elif [[ -d spec ]]; then
      run "rb: test (rspec)$sfx" bundle exec rspec --no-color
    else
      skip_msg "rb: test$sfx (no rake test task or spec/)"
    fi
  fi

  # Rust: cargo's own check subcommands are the package-manager-native checks.
  if [[ -f Cargo.toml ]]; then
    run "rs: check$sfx" cargo check --quiet
    run "rs: clippy$sfx" cargo clippy --quiet -- -D warnings
    run "rs: fmt-check$sfx" cargo fmt --check
    run "rs: test$sfx" cargo test --quiet
  fi

  # Go: the go toolchain's own check subcommands.
  if [[ -f go.mod ]]; then
    run "go: vet$sfx" go vet ./...
    run "go: test$sfx" go test ./...
  fi
}

check_dir "$root" ""
for sub in "$root"/*/; do
  check_dir "$sub" " [$(basename "$sub")]"
done

echo ""
echo "checks: $pass passed, $fail failed, $skip skipped"
exit "$fail"
