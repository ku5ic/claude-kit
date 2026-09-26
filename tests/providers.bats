#!/usr/bin/env bats
# Tests for bin/_lib.sh's extractors, kit_tasks, and kit_subprojects: the
# task_providers, checks, and subproject data layer in kit.yml.
#
# The real kit.yml is used; HOME is faked so its caches land in the tmpdir.

setup() {
  export HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$HOME"
  unset CLAUDE_PLUGIN_ROOT CLAUDE_CONFIG_DIR
  # shellcheck source=../bin/_lib.sh
  source "$BATS_TEST_DIRNAME/../bin/_lib.sh"
  F="$BATS_TEST_TMPDIR/fixture"
  mkdir -p "$F"
}

# The cache build: one yq pass fills every list, so an expression yq rejects
# empties all of them at once, guard lists included. This pins that the real
# kit.yml builds every one.

@test "the real kit.yml builds every cached list" {
  kit_stacks_load
  local name
  for name in STACK_SENTINELS_FULL STACK_SENTINEL_STACKS STACK_PM_LOCKFILES \
    KIT_GUARDED_LOCKFILES KIT_PROTECTED_BRANCHES KIT_RC_FILES KIT_SENSITIVE_PATHS \
    KIT_TP_NAMES KIT_CHECK_NAMES KIT_TC_STACKS KIT_ORCH_NAMES KIT_TOOLS KIT_FMT_NAMES; do
    local -n list="$name"
    ((${#list[@]} > 0)) || {
      echo "empty: $name"
      return 1
    }
    unset -n list
  done
}

# extractors

@test "json_keys lists an object's keys in file order" {
  printf '{"scripts":{"test":"x","lint":"y","build":"z"}}\n' >"$F/package.json"
  run json_keys "$F/package.json" .scripts
  [ "$output" = "test
lint
build" ]
}

@test "json_keys prints nothing for a missing path or file" {
  printf '{}\n' >"$F/package.json"
  run json_keys "$F/package.json" .scripts
  [ -z "$output" ]
  run json_keys "$F/nope.json" .scripts
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "json_array reads a nested array" {
  printf '{"workspaces":{"packages":["apps/*","packages/*"]}}\n' >"$F/package.json"
  run json_array "$F/package.json" .workspaces.packages
  [ "$output" = "apps/*
packages/*" ]
}

@test "json_array ignores a non-array at the path" {
  printf '{"workspaces":{"packages":["a"]}}\n' >"$F/package.json"
  run json_array "$F/package.json" .workspaces
  [ -z "$output" ]
}

@test "toml_keys reads a nested table" {
  printf '[tool.poe.tasks]\ntest = "pytest"\nlint = "ruff check"\n' >"$F/pyproject.toml"
  run toml_keys "$F/pyproject.toml" .tool.poe.tasks
  [ "$output" = "test
lint" ]
}

@test "toml_array reads a workspace members list" {
  printf '[workspace]\nmembers = ["crates/a", "crates/b"]\n' >"$F/Cargo.toml"
  run toml_array "$F/Cargo.toml" .workspace.members
  [ "$output" = "crates/a
crates/b" ]
}

@test "yaml_array reads pnpm workspace packages" {
  printf 'packages:\n  - "packages/*"\n  - apps/web\n' >"$F/pnpm-workspace.yaml"
  run yaml_array "$F/pnpm-workspace.yaml" .packages
  [ "$output" = "packages/*
apps/web" ]
}

@test "make_targets skips .PHONY, pattern rules, and := assignments" {
  printf '.PHONY: test\nVAR := 1\ntest: build\n\tgo test\nbuild:\n\tgo build\n%%.o: %%.c\n\tcc\n' >"$F/Makefile"
  run make_targets "$F/Makefile"
  [ "$output" = "test
build" ]
}

@test "just_recipes falls back to recipe header lines without just" {
  command -v just >/dev/null && skip "just is installed; this covers the fallback"
  printf 'set shell := ["bash", "-c"]\nversion := "1"\n\ntest *args:\n  cargo test {{args}}\n@lint:\n  cargo clippy\n' >"$F/justfile"
  run just_recipes "$F/justfile"
  [ "$output" = "test
lint" ]
}

@test "regex_lines prints the last capture group of each match" {
  printf 'task :build do\nend\n  task "db:migrate" do\nputs 1\n' >"$F/Rakefile"
  run regex_lines "$F/Rakefile" '^[[:space:]]*task[[:space:]]+:?"?([A-Za-z0-9_:]+)'
  [ "$output" = "build
db:migrate" ]
}

@test "an unknown extractor name is refused, not run" {
  run _kit_extract rm "$F/x" y
  [ -z "${output##*unknown extractor in kit.yml: rm*}" ]
}

# kit_tasks

@test "kit_tasks resolves {pm} from the lockfile" {
  git init -q -b main "$F"
  printf '{"scripts":{"test":"vitest"}}\n' >"$F/package.json"
  touch "$F/pnpm-lock.yaml"
  run kit_tasks "$F"
  [ "$output" = "package-scripts	js	test	pnpm run test" ]
}

@test "kit_tasks defaults {pm} to npm with no lockfile" {
  printf '{"scripts":{"lint":"eslint ."}}\n' >"$F/package.json"
  run kit_tasks "$F"
  [ "$output" = "package-scripts	js	lint	npm run lint" ]
}

@test "kit_tasks uses run_by_pm for poe under poetry" {
  printf '[tool.poe.tasks]\ntest = "pytest"\n' >"$F/pyproject.toml"
  touch "$F/poetry.lock"
  run kit_tasks "$F"
  [ "$output" = "poe	python	test	poetry run poe test" ]
}

@test "kit_tasks covers several providers in one directory" {
  printf 'test:\n\tgo test\n' >"$F/Makefile"
  printf '[tool.pdm.scripts]\nlint = "ruff"\n' >"$F/pyproject.toml"
  run kit_tasks "$F"
  [[ "$output" == *"make	-	test	make test"* ]]
  [[ "$output" == *"pdm	python	lint	pdm run lint"* ]]
}

# kit_subprojects

# pnpm root with a workspace package at packages/a, a uv project at
# services/api, and an untracked node_modules manifest that must not count.
make_monorepo() {
  git init -q -b main "$F"
  printf '{"name":"root","private":true}\n' >"$F/package.json"
  printf 'packages:\n  - "packages/*"\n' >"$F/pnpm-workspace.yaml"
  touch "$F/pnpm-lock.yaml"
  mkdir -p "$F/packages/a" "$F/services/api" "$F/node_modules/dep"
  printf '{"name":"a","scripts":{"test":"vitest"}}\n' >"$F/packages/a/package.json"
  printf '[project]\nname = "api"\n' >"$F/services/api/pyproject.toml"
  touch "$F/services/api/uv.lock"
  printf '{"name":"dep"}\n' >"$F/node_modules/dep/package.json"
  git -C "$F" add package.json pnpm-workspace.yaml pnpm-lock.yaml packages services
}

@test "kit_subprojects finds the root, a workspace package, and a nested project" {
  make_monorepo
  run kit_subprojects "$F"
  [ "$status" -eq 0 ]
  [ "$output" = ".
packages/a
services/api" ]
}

@test "kit_subprojects defaults to the git toplevel of the cwd" {
  make_monorepo
  cd "$F/services/api"
  run kit_subprojects
  [ "$output" = ".
packages/a
services/api" ]
}

@test "kit_subprojects respects subproject_max_depth" {
  git init -q -b main "$F"
  mkdir -p "$F/a/b/c/d/e"
  printf '{}\n' >"$F/a/b/c/d/package.json"
  printf '{}\n' >"$F/a/b/c/d/e/package.json"
  git -C "$F" add a
  run kit_subprojects "$F"
  [ "$output" = ".
a/b/c/d" ]
}

# kit_args

@test "kit_args skips options and keeps everything after --" {
  run kit_args '-n -- -weird-name file.txt'
  [ "$output" = "-weird-name
file.txt" ]
}

@test "kit_args never glob-expands a word against the cwd" {
  cd "$F"
  touch a.txt b.txt
  run kit_args '-v *.txt'
  [ "$output" = "*.txt" ]
}

@test "kit_subprojects reads go.work and Cargo workspace members" {
  git init -q -b main "$F"
  mkdir -p "$F/svc" "$F/tools" "$F/crates/x"
  printf 'go 1.22\n\nuse (\n\t./svc\n)\nuse ./tools\n' >"$F/go.work"
  printf '[workspace]\nmembers = ["crates/*"]\n' >"$F/Cargo.toml"
  run kit_subprojects "$F"
  [ "$output" = ".
crates/x
svc
tools" ]
}
