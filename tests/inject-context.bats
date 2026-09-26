#!/usr/bin/env bats
# Tests for ~/.dotfiles/claude-kit/hooks/inject-context.sh.
#
# inject-context.sh calls project-name.sh/project-root.sh via absolute
# $HOME-prefixed paths, so each test fakes $HOME with tiny stand-in scripts
# for those two collaborators (echoing a fixed name/root) rather than
# exercising the real sentinel walk -- this isolates inject-context.sh's own
# logic, which is what this suite covers. The real bin/_lib.sh is still
# sourced (relative to the script's own location), so
# stack-cache and skill-derivation logic is real; only the two collaborator
# scripts and the data files under $HOME are fixtures.
#
# Run with: bats tests/

load helper

setup() {
  HOOK="$BATS_TEST_DIRNAME/../hooks/inject-context.sh"
  LIB="$BATS_TEST_DIRNAME/../bin/_lib.sh"
  kit_test_home --plugin-root
  FAKE_ROOT="$BATS_TEST_TMPDIR/project"
  mkdir -p "$FAKE_HOME/.claude/bin" "$FAKE_HOME/.claude/logs" "$FAKE_HOME/.claude/scratch" "$FAKE_ROOT"
  # Real git init: a non-git root breaks inject-context.sh entirely (see the
  # dedicated RISK test below), which would otherwise block every other test.
  git -C "$FAKE_ROOT" init -q
  # check_prereqs wants the kit rules linked; without this every test would
  # get the warning JSON instead of the context.
  mkdir -p "$FAKE_HOME/.claude/rules"
  ln -s "$BATS_TEST_DIRNAME/../rules" "$FAKE_HOME/.claude/rules/kit"
  # Likewise a readable kit.yml; tests that need content overwrite it.
  touch "$FAKE_HOME/.claude/kit.yml"

  set_project_name "testproject"
  set_project_root "$FAKE_ROOT"

  cache_path="$(HOME="$FAKE_HOME" bash -c "source '$LIB' >/dev/null 2>&1; stack_cache_file 'testproject' '$FAKE_ROOT'")"
  mkdir -p "$(dirname "$cache_path")"
}

set_project_name() {
  local name="$1"
  cat >"$FAKE_HOME/.claude/bin/project-name.sh" <<EOF
#!/usr/bin/env bash
echo "$name"
EOF
  chmod +x "$FAKE_HOME/.claude/bin/project-name.sh"
}

set_project_root() {
  local root="$1"
  cat >"$FAKE_HOME/.claude/bin/project-root.sh" <<EOF
#!/usr/bin/env bash
echo "$root"
EOF
  chmod +x "$FAKE_HOME/.claude/bin/project-root.sh"
}

write_kit_yml() {
  cat >"$FAKE_HOME/.claude/kit.yml"
}

write_cache() {
  printf '%s\n' "$@" >"$cache_path"
}

# run_inject_context <session_id> [cwd]
run_inject_context() {
  local session="$1" cwd="${2:-$FAKE_ROOT}"
  jq -n --arg sess "$session" --arg cwd "$cwd" '{session_id: $sess, cwd: $cwd}' |
    HOME="$FAKE_HOME" "$HOOK"
}

@test "<required-skills> contains every global_skills entry" {
  write_kit_yml <<'YAML'
global_skills:
  - fix-sizing
  - context-gathering
skill_triggers: {}
stacks: {}
YAML
  write_cache "root: $FAKE_ROOT" "js: yes"

  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  [[ "$output" == *"<required-skills>"* ]]
  [[ "$output" == *"fix-sizing"* ]]
  [[ "$output" == *"context-gathering"* ]]
}

@test "<suggested-skills> has one line per detected stack skill with its trigger phrase" {
  write_kit_yml <<'YAML'
global_skills:
  - fix-sizing
skill_triggers:
  react-patterns: "before building or restructuring React components"
stacks:
  js:
    skills: [javascript-patterns]
    extras:
      - name: react
        dep: react
        skills: [react-patterns]
YAML
  write_cache "root: $FAKE_ROOT" "js: yes (react)"

  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  [[ "$output" == *"<suggested-skills>"* ]]
  [[ "$output" == *"before building or restructuring React components: load react-patterns via the Skill tool"* ]]
  [[ "$output" == *"load javascript-patterns via the Skill tool"* ]]
}

@test "a repo with no sentinel produces no injection" {
  set_project_name "unknown"
  write_kit_yml <<'YAML'
global_skills:
  - fix-sizing
YAML

  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "a suggested skill logs a suggested-skill marker to skills.jsonl" {
  write_kit_yml <<'YAML'
global_skills: []
skill_triggers:
  react-patterns: "before building or restructuring React components"
stacks:
  js:
    skills: []
    extras:
      - name: react
        dep: react
        skills: [react-patterns]
YAML
  write_cache "root: $FAKE_ROOT" "js: yes (react)"

  run run_inject_context "s1"
  [ "$status" -eq 0 ]

  log_file="$FAKE_HOME/.claude/logs/skills.jsonl"
  [ -f "$log_file" ]
  run jq -s '[.[] | select(.event == "suggested-skill" and .session_id == "s1" and .skill_file == "react-patterns")] | length' "$log_file"
  [ "$output" -eq 1 ]
}

# Regression coverage for a fixed bug: inject-context.sh's dirty-file count
# runs `git -C "$project_root" status --porcelain | wc -l | tr -d ' '`. Under
# pipefail, a non-git project_root used to make that pipeline fail, and the
# fail-open ERR trap from kit_hook_init turned that into a silent early exit --
# no repo-context, no required/suggested skills, no marker touch. Fixed by
# falling back to a "dirty-files: unknown" line instead of aborting.
@test "a non-git project root degrades to dirty-files: unknown instead of failing open" {
  non_git_root="$BATS_TEST_TMPDIR/non-git-project"
  mkdir -p "$non_git_root"
  set_project_root "$non_git_root"
  write_kit_yml <<'YAML'
global_skills:
  - fix-sizing
YAML
  cache_path="$(HOME="$FAKE_HOME" bash -c "source '$LIB' >/dev/null 2>&1; stack_cache_file 'testproject' '$non_git_root'")"
  mkdir -p "$(dirname "$cache_path")"
  write_cache "root: $non_git_root" "js: yes"

  run run_inject_context "s1" "$non_git_root"
  [ "$status" -eq 0 ]
  [[ "$output" == *"<required-skills>"* ]]
  [[ "$output" == *"fix-sizing"* ]]
  [[ "$output" == *"dirty-files (at session start): unknown"* ]]
}

# check_prereqs: guards fail open without these, so the hook warns instead.

# PATH holding only what check_prereqs touches before it exits, plus $@.
restricted_path() {
  local dir="$BATS_TEST_TMPDIR/restricted-bin" tool
  mkdir -p "$dir"
  for tool in dirname grep "$@"; do
    ln -sf "$(command -v "$tool")" "$dir/$tool"
  done
  printf '%s' "$dir"
}

@test "prereqs: a non-mikefarah yq on PATH gets a warning" {
  local fake="$BATS_TEST_TMPDIR/fake-yq"
  mkdir -p "$fake"
  printf '#!/bin/sh\necho "yq 3.4.3"\n' >"$fake/yq"
  chmod +x "$fake/yq"

  run bash -c "echo '{}' | HOME='$FAKE_HOME' PATH='$fake:$PATH' '$HOOK'"
  [ "$status" -eq 0 ]
  [[ "$output" == *'"systemMessage"'* ]]
  [[ "$output" == *"mikefarah yq"* ]]
  [[ "$output" != *"<repo-context>"* ]]
}

@test "prereqs: missing jq prints valid JSON naming jq" {
  local bash_bin
  bash_bin="$(command -v bash)"
  run env HOME="$FAKE_HOME" PATH="$(restricted_path yq)" "$bash_bin" "$HOOK" </dev/null
  [ "$status" -eq 0 ]
  printf '%s' "$output" | jq -e '.systemMessage | test("jq \\(brew install jq\\)")'
}

@test "prereqs: kit rules not linked under ~/.claude/rules gets a warning" {
  rm "$FAKE_HOME/.claude/rules/kit"

  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  [[ "$output" == *"run bootstrap.sh"* ]]
}

@test "prereqs: a missing kit.yml gets a warning naming bootstrap.sh" {
  rm "$FAKE_HOME/.claude/kit.yml"

  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  [[ "$output" == *"readable kit.yml"* ]]
  [[ "$output" == *"run bootstrap.sh"* ]]
}

@test "prereqs: kit rules linked under another name count" {
  mv "$FAKE_HOME/.claude/rules/kit" "$FAKE_HOME/.claude/rules/claude-kit"
  write_kit_yml <<'YAML'
global_skills:
  - fix-sizing
YAML
  write_cache "root: $FAKE_ROOT" "js: yes"

  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  [[ "$output" != *"systemMessage"* ]]
  [[ "$output" == *"<required-skills>"* ]]
}

@test "prereqs: another copy of the kit's rules counts (plugin cache vs marketplace clone)" {
  rm "$FAKE_HOME/.claude/rules/kit"
  cp -R "$BATS_TEST_DIRNAME/../rules" "$BATS_TEST_TMPDIR/clone-rules"
  ln -s "$BATS_TEST_TMPDIR/clone-rules" "$FAKE_HOME/.claude/rules/claude-kit"

  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  [[ "$output" != *"the kit rules linked"* ]]
}

@test "prereqs: a rules dir missing one of the kit's rule files doesn't count" {
  rm "$FAKE_HOME/.claude/rules/kit"
  cp -R "$BATS_TEST_DIRNAME/../rules" "$BATS_TEST_TMPDIR/partial-rules"
  rm "$BATS_TEST_TMPDIR/partial-rules/workflow.md"
  ln -s "$BATS_TEST_TMPDIR/partial-rules" "$FAKE_HOME/.claude/rules/claude-kit"

  run run_inject_context "s1"
  [[ "$output" == *"the kit rules linked"* ]]
}

@test "prereqs: bash older than 4.4 gets a warning" {
  [[ -x /bin/bash ]] && [[ "$(/bin/bash -c 'echo ${BASH_VERSINFO[0]}')" -lt 4 ]] || skip "no bash 3.x at /bin/bash"
  run env HOME="$FAKE_HOME" /bin/bash "$HOOK" </dev/null
  [ "$status" -eq 0 ]
  [[ "$output" == *"bash 4.4+"* ]]
}

# <tooling>: run forms from kit.yml's task_providers and toolchain_checks.

# The real kit.yml, so the production provider tables are exercised.
use_real_kit_yml() {
  cp "$BATS_TEST_DIRNAME/../kit.yml" "$FAKE_HOME/.claude/kit.yml"
}

# The <tooling> block alone, from the first line to the closing tag.
tooling_block() {
  printf '%s\n' "$output" | sed -n '/^<tooling>$/,/^<\/tooling>$/p'
}

@test "tooling: a Python project's justfile recipes are listed as just <recipe>" {
  use_real_kit_yml
  printf '[project]\nname = "x"\n' >"$FAKE_ROOT/pyproject.toml"
  printf 'test:\n  pytest\nlint:\n  ruff check\n' >"$FAKE_ROOT/justfile"
  git -C "$FAKE_ROOT" add -A

  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  [[ "$(tooling_block)" == *"tasks:
  just test
  just lint"* ]]
}

@test "tooling: a Rust project lists only its toolchain checks" {
  use_real_kit_yml
  printf '[package]\nname = "x"\n' >"$FAKE_ROOT/Cargo.toml"
  git -C "$FAKE_ROOT" add -A

  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  [ "$(tooling_block | grep '^  ')" = "  cargo check
  cargo clippy -- -D warnings
  cargo fmt --check
  cargo test" ]
}

@test "tooling: an OpenTofu project gets {bin} filled, and validate only after init" {
  use_real_kit_yml
  touch "$FAKE_ROOT/.terraform.lock.hcl"
  git -C "$FAKE_ROOT" add -A
  mkdir -p "$BATS_TEST_TMPDIR/stubs"
  printf '#!/bin/sh\n' >"$BATS_TEST_TMPDIR/stubs/tofu"
  chmod +x "$BATS_TEST_TMPDIR/stubs/tofu"

  PATH="$BATS_TEST_TMPDIR/stubs:$PATH" run run_inject_context "s1"
  [ "$(tooling_block | grep '^  ')" = "  tofu fmt -check -recursive" ]

  mkdir "$FAKE_ROOT/.terraform"
  PATH="$BATS_TEST_TMPDIR/stubs:$PATH" run run_inject_context "s2"
  [ "$(tooling_block | grep '^  ')" = "  tofu fmt -check -recursive
  tofu validate" ]
}

@test "tooling: workspace packages get their own section with the root's package manager" {
  use_real_kit_yml
  printf '{"name":"root","scripts":{"lint":"eslint ."}}\n' >"$FAKE_ROOT/package.json"
  printf 'packages:\n  - "packages/*"\n' >"$FAKE_ROOT/pnpm-workspace.yaml"
  touch "$FAKE_ROOT/pnpm-lock.yaml"
  mkdir -p "$FAKE_ROOT/packages/a"
  printf '{"name":"a","scripts":{"test":"vitest"}}\n' >"$FAKE_ROOT/packages/a/package.json"
  git -C "$FAKE_ROOT" add -A

  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  local block
  block="$(tooling_block)"
  [[ "$block" == *"package-manager: pnpm"* ]]
  [[ "$block" == *"tasks:
  pnpm run lint"* ]]
  [[ "$block" == *"tasks [packages/a]:
  pnpm run test"* ]]
  [[ "$block" == *"guidance: "* ]]
}

@test "tooling: a project with no providers or toolchain gets tools only, no tasks or guidance" {
  use_real_kit_yml
  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  local block
  block="$(tooling_block)"
  [[ "$block" != *"tasks:"* ]]
  [[ "$block" != *"guidance:"* ]]
  [[ "$block" == *"available: "* || "$block" == *"missing: "* ]]
}

@test "tooling: tools are split into available and missing by PATH" {
  cat >"$FAKE_HOME/.claude/kit.yml" <<'YAML'
tools: [jq, definitely-not-a-real-tool-xyz, git]
YAML
  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  [[ "$(tooling_block)" == *"available: jq, git"* ]]
  [[ "$(tooling_block)" == *"missing: definitely-not-a-real-tool-xyz"* ]]
}

@test "tooling: no tools and no tasks means no block" {
  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  [[ "$output" != *"<tooling>"* ]]
}

@test "repo-context names the scratch dir without creating it" {
  write_cache "root: $FAKE_ROOT" "js: yes"
  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  local root
  root="$(cd -P "$FAKE_ROOT" && pwd)"
  [[ "$output" == *"scratch: $root/.claude/scratch"* ]]
  [ ! -e "$FAKE_ROOT/.claude/scratch" ]
}
