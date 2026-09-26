#!/usr/bin/env bats
# Tests for ~/.dotfiles/claude-kit/hooks/stop-checks.sh.
#
# stop-checks.sh calls run-checks.sh via $CLAUDE_PLUGIN_ROOT, so each test
# fakes it with a stub that passes or fails on demand. Each test writes a
# transcript JSONL describing the turn the hook inspects.
#
# Run with: bats tests/

load helper

setup() {
  HOOK="$BATS_TEST_DIRNAME/../hooks/stop-checks.sh"
  kit_test_home --plugin-root
  REPO="$BATS_TEST_TMPDIR/repo"
  TRANSCRIPT="$BATS_TEST_TMPDIR/transcript.jsonl"
  mkdir -p "$FAKE_HOME/.claude/bin" "$REPO"
  git -C "$REPO" init -q
  git -C "$REPO" -c user.email=t@t -c user.name=t commit -q --allow-empty -m init
  echo x >"$REPO/a"
  set_checks pass
}

set_checks() {
  if [[ "$1" == pass ]]; then
    printf '#!/usr/bin/env bash\necho "PASS lint"\necho "checks: 1 passed, 0 failed, 0 skipped"\n' >"$FAKE_HOME/.claude/bin/run-checks.sh"
  else
    printf '#!/usr/bin/env bash\necho "FAIL lint (make lint)"\necho "noise"\necho "checks: 0 passed, 1 failed, 0 skipped"\nexit 1\n' >"$FAKE_HOME/.claude/bin/run-checks.sh"
  fi
  chmod +x "$FAKE_HOME/.claude/bin/run-checks.sh"
}

# turn <tool>...  appends a real user prompt, then one assistant tool_use per arg.
turn() {
  jq -nc '{type:"user", message:{content:"do it"}}' >>"$TRANSCRIPT"
  local tool
  for tool in "$@"; do
    jq -nc --arg n "$tool" '{type:"assistant", message:{content:[{type:"tool_use", name:$n}]}}' >>"$TRANSCRIPT"
    jq -nc '{type:"user", message:{content:[{type:"tool_result"}]}}' >>"$TRANSCRIPT"
  done
}

stop() {
  local active="${1:-false}"
  jq -nc --arg cwd "$REPO" --arg t "$TRANSCRIPT" --argjson active "$active" \
    '{hook_event_name:"Stop", session_id:"s1", cwd:$cwd, transcript_path:$t, stop_hook_active:$active}' |
    HOME="$FAKE_HOME" "$HOOK"
}

@test "missing transcript runs nothing" {
  set_checks fail
  run stop
  [ "$status" -eq 0 ]
}

@test "stop_hook_active lets the stop through even when checks would fail" {
  turn Edit
  set_checks fail
  run stop true
  [ "$status" -eq 0 ]
}

@test "an edit with passing checks allows the stop" {
  turn Read Edit
  run stop
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS lint"* ]]
}

@test "an edit with failing checks blocks with the FAIL lines only" {
  turn Write
  set_checks fail
  run stop
  [ "$status" -eq 2 ]
  [[ "$output" == *"FAIL lint"* ]]
  [[ "$output" != *"noise"* ]]
}

@test "edits committed in the turn skip the checks" {
  turn Edit Bash
  git -C "$REPO" add a
  git -C "$REPO" -c user.email=t@t -c user.name=t commit -q -m edit
  set_checks fail
  run stop
  [ "$status" -eq 0 ]
}

@test "a turn without edit tools skips the checks" {
  turn Read Bash
  set_checks fail
  run stop
  [ "$status" -eq 0 ]
}

@test "an edit in an earlier turn does not count" {
  turn Edit
  turn Read
  set_checks fail
  run stop
  [ "$status" -eq 0 ]
}

@test "a meta user entry does not start a new turn" {
  turn Edit
  jq -nc '{type:"user", isMeta:true, message:{content:"skill loaded"}}' >>"$TRANSCRIPT"
  set_checks fail
  run stop
  [ "$status" -eq 2 ]
}

@test "outside a git worktree exits clean" {
  REPO="$BATS_TEST_TMPDIR/plain"
  mkdir -p "$REPO"
  turn Edit
  set_checks fail
  run stop
  [ "$status" -eq 0 ]
}

# Scoping: only the subprojects holding an edited file are checked.

# turn_edit <tool> <path>...  a user prompt, then one edit per path.
turn_edit() {
  local tool="$1" path
  shift
  jq -nc '{type:"user", message:{content:"do it"}}' >>"$TRANSCRIPT"
  for path in "$@"; do
    jq -nc --arg n "$tool" --arg p "$path" \
      '{type:"assistant", message:{content:[{type:"tool_use", name:$n, input:{file_path:$p}}]}}' >>"$TRANSCRIPT"
    jq -nc '{type:"user", message:{content:[{type:"tool_result"}]}}' >>"$TRANSCRIPT"
  done
}

# A passing run-checks.sh stub that records its arguments.
set_checks_recording() {
  printf '#!/usr/bin/env bash\necho "$*" >"%s"\necho "checks: 0 passed, 0 failed, 0 skipped"\n' \
    "$BATS_TEST_TMPDIR/args" >"$FAKE_HOME/.claude/bin/run-checks.sh"
  chmod +x "$FAKE_HOME/.claude/bin/run-checks.sh"
}

# The real kit.yml, and REPO as a pnpm root with packages/a and a Python
# service at services/api, all tracked.
make_monorepo() {
  cp "$BATS_TEST_DIRNAME/../kit.yml" "$FAKE_HOME/.claude/kit.yml"
  REPO="$(cd -P "$REPO" && pwd)"
  printf '{"name":"root","private":true}\n' >"$REPO/package.json"
  printf 'packages:\n  - "packages/*"\n' >"$REPO/pnpm-workspace.yaml"
  touch "$REPO/pnpm-lock.yaml"
  mkdir -p "$REPO/packages/a" "$REPO/services/api"
  printf '{"name":"a","scripts":{"test":"vitest"}}\n' >"$REPO/packages/a/package.json"
  printf '[tool.pdm.scripts]\ntest = "pytest"\n' >"$REPO/services/api/pyproject.toml"
  git -C "$REPO" add package.json pnpm-workspace.yaml pnpm-lock.yaml packages services
}

@test "an edit in a subproject checks only that subproject" {
  make_monorepo
  set_checks_recording
  turn_edit Edit "$REPO/services/api/app.py"
  run stop
  [ "$status" -eq 0 ]
  [ "$(cat "$BATS_TEST_TMPDIR/args")" = "--only services/api" ]
}

@test "an edit at the root, in no subproject, checks only the root" {
  make_monorepo
  set_checks_recording
  turn_edit Write "$REPO/README.md"
  run stop
  [ "$(cat "$BATS_TEST_TMPDIR/args")" = "--only ." ]
}

@test "edits in two subprojects check both, each once" {
  make_monorepo
  set_checks_recording
  turn_edit Edit "$REPO/packages/a/index.ts" "$REPO/services/api/app.py" "$REPO/packages/a/util.ts"
  run stop
  [ "$(cat "$BATS_TEST_TMPDIR/args")" = "--only packages/a services/api" ]
}

@test "an edit outside the repo checks nothing" {
  make_monorepo
  set_checks_recording
  turn_edit Write "$BATS_TEST_TMPDIR/elsewhere/notes.md"
  run stop
  [ "$status" -eq 0 ]
  [ ! -e "$BATS_TEST_TMPDIR/args" ]
}

@test "the real run-checks.sh prints no packages/a labels after an edit in services/api" {
  make_monorepo
  ln -sf "$BATS_TEST_DIRNAME/../bin/run-checks.sh" "$FAKE_HOME/.claude/bin/run-checks.sh"
  ln -sf "$BATS_TEST_DIRNAME/../bin/_lib.sh" "$FAKE_HOME/.claude/bin/_lib.sh"
  local stubs="$BATS_TEST_TMPDIR/stubs"
  mkdir -p "$stubs"
  printf '#!/bin/sh\nexit 0\n' >"$stubs/pdm"
  printf '#!/bin/sh\nexit 0\n' >"$stubs/pnpm"
  chmod +x "$stubs/pdm" "$stubs/pnpm"
  turn_edit Edit "$REPO/services/api/app.py"

  PATH="$stubs:$PATH" run stop
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS python: test (test) [services/api]"* ]]
  [[ "$output" != *"packages/a"* ]]
}
