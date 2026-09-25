#!/usr/bin/env bats
# Tests for ~/.dotfiles/claude-kit/bin/subagent-statusline.sh.
#
# Claude Code runs subagentStatusLine once per render with {columns, tasks:[...]}
# on stdin, and parses stdout as one {"id","content"} JSON object per line.
# Tests therefore assert on the decoded content field, not on raw stdout, and
# cover the version-gated fields (model, contextWindowSize, effort) that older
# builds omit.
#
# Run with: bats tests/

# First test file here to use `run` flags (--separate-stderr), which bats only
# guarantees from 1.5.0; without this it warns BW02 on every run.
bats_require_minimum_version 1.5.0

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../bin/subagent-statusline.sh"
}

# envelope <task-json>...  wraps task objects in the payload Claude Code sends
envelope() {
  local tasks
  tasks="$(
    IFS=,
    printf '%s' "$*"
  )"
  printf '{"session_id":"s1","columns":120,"tasks":[%s]}' "$tasks"
}

run_subagent_statusline() {
  printf '%s' "$1" | bash "$SCRIPT"
}

# content_for <stdout> <task-id>  decodes the content rendered for one task
content_for() {
  jq -r --arg id "$2" 'select(.id == $id) | .content' <<<"$1"
}

@test "emits one JSON object per task, keyed by task id" {
  run run_subagent_statusline "$(envelope \
    '{"id":"t1","name":"scout","status":"running"}' \
    '{"id":"t2","name":"tester","status":"pending"}')"
  [ "$status" -eq 0 ]
  [ "$(printf '%s\n' "$output" | wc -l | tr -d ' ')" -eq 2 ]
  [ "$(content_for "$output" t1)" = "scout [running]" ]
  [ "$(content_for "$output" t2)" = "tester [pending]" ]
}

@test "every emitted line is JSON with a string id and string content" {
  run run_subagent_statusline "$(envelope '{"id":"t1","name":"scout","status":"running"}')"
  run jq -e 'type == "object" and (.id | type) == "string" and (.content | type) == "string"' <<<"$output"
  [ "$status" -eq 0 ]
}

@test "renders model when present" {
  run run_subagent_statusline "$(envelope '{"id":"t1","name":"scout","status":"running","model":"claude-opus-4-8"}')"
  [ "$(content_for "$output" t1)" = "scout [running]  claude-opus-4-8" ]
}

@test "omits model when absent (pre-v2.1.205 payload)" {
  run run_subagent_statusline "$(envelope '{"id":"t1","name":"scout","status":"running"}')"
  [[ "$(content_for "$output" t1)" != *"claude-"* ]]
}

@test "renders effort when present" {
  run run_subagent_statusline "$(envelope '{"id":"t1","name":"scout","status":"running","effort":"high"}')"
  [[ "$(content_for "$output" t1)" == *"effort:high"* ]]
}

@test "omits effort when absent (pre-v2.1.214 payload)" {
  run run_subagent_statusline "$(envelope '{"id":"t1","name":"scout","status":"running"}')"
  [[ "$(content_for "$output" t1)" != *"effort:"* ]]
}

@test "renders a numeric token-budget effort value verbatim" {
  run run_subagent_statusline "$(envelope '{"id":"t1","name":"scout","status":"running","effort":12000}')"
  [[ "$(content_for "$output" t1)" == *"effort:12000"* ]]
}

@test "renders context percentage from tokenCount over contextWindowSize" {
  run run_subagent_statusline "$(envelope '{"id":"t1","name":"scout","status":"running","contextWindowSize":200000,"tokenCount":45000}')"
  [[ "$(content_for "$output" t1)" == *" 22%"* ]]
}

@test "omits context percentage when contextWindowSize is absent" {
  run run_subagent_statusline "$(envelope '{"id":"t1","name":"scout","status":"running","tokenCount":45000}')"
  [[ "$(content_for "$output" t1)" != *"%"* ]]
}

@test "omits context percentage when tokenCount is absent" {
  run run_subagent_statusline "$(envelope '{"id":"t1","name":"scout","status":"running","contextWindowSize":200000}')"
  [[ "$(content_for "$output" t1)" != *"%"* ]]
}

@test "omits context percentage when contextWindowSize is explicitly zero (no divide by zero)" {
  run run_subagent_statusline "$(envelope '{"id":"t1","name":"scout","status":"running","contextWindowSize":0,"tokenCount":5000}')"
  [ "$status" -eq 0 ]
  [[ "$(content_for "$output" t1)" != *"%"* ]]
}

@test "renders zero percent when tokenCount is zero" {
  run run_subagent_statusline "$(envelope '{"id":"t1","name":"scout","status":"running","contextWindowSize":200000,"tokenCount":0}')"
  [[ "$(content_for "$output" t1)" == *" 0%"* ]]
}

@test "full payload renders every segment in order" {
  run run_subagent_statusline "$(envelope '{"id":"t1","name":"scout","status":"running","model":"claude-opus-4-8","contextWindowSize":200000,"tokenCount":45000,"effort":"high"}')"
  [ "$(content_for "$output" t1)" = "scout [running]  claude-opus-4-8  effort:high  22%" ]
}

@test "falls back to label when name is absent" {
  run run_subagent_statusline "$(envelope '{"id":"t1","label":"explore repo","status":"running"}')"
  [ "$(content_for "$output" t1)" = "explore repo [running]" ]
}

@test "falls back to description when name and label are absent" {
  run run_subagent_statusline "$(envelope '{"id":"t1","description":"find the bug","status":"running"}')"
  [ "$(content_for "$output" t1)" = "find the bug [running]" ]
}

@test "falls back to a placeholder when name, label and description are all absent" {
  run run_subagent_statusline "$(envelope '{"id":"t1","status":"running"}')"
  [ "$(content_for "$output" t1)" = "task [running]" ]
}

@test "skips a task with no id, since the id keys the render" {
  run run_subagent_statusline "$(envelope \
    '{"name":"orphan","status":"running"}' \
    '{"id":"t2","name":"tester","status":"running"}')"
  [ "$status" -eq 0 ]
  [ "$(printf '%s\n' "$output" | wc -l | tr -d ' ')" -eq 1 ]
  [ "$(content_for "$output" t2)" = "tester [running]" ]
}

@test "empty task list produces no output" {
  run run_subagent_statusline '{"columns":120,"tasks":[]}'
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "payload without a tasks key produces no output" {
  run run_subagent_statusline '{"columns":120}'
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# Claude Code reads stdout and ignores stderr on a zero exit, so jq's parse
# error is allowed to surface for anyone running this by hand; only stdout has
# to stay empty.
@test "invalid JSON exits clean with empty stdout rather than a partial line" {
  run --separate-stderr run_subagent_statusline 'not json'
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "missing jq exits clean with no output" {
  local stub_dir="$BATS_TEST_TMPDIR/stub_no_jq"
  mkdir -p "$stub_dir"
  ln -s "$(command -v cat)" "$stub_dir/cat"
  local bash_bin
  bash_bin="$(command -v bash)"
  run env PATH="$stub_dir" "$bash_bin" "$SCRIPT" <<<'{"columns":120,"tasks":[{"id":"t1","name":"scout"}]}'
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}
