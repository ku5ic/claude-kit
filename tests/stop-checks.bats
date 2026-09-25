#!/usr/bin/env bats
# Tests for ~/.dotfiles/claude-kit/hooks/stop-checks.sh.
#
# stop-checks.sh calls run-checks.sh via $CLAUDE_PLUGIN_ROOT, so each test
# fakes it with a stub that passes or fails on demand. Each test writes a
# transcript JSONL describing the turn the hook inspects.
#
# Run with: bats tests/

setup() {
  HOOK="$BATS_TEST_DIRNAME/../hooks/stop-checks.sh"
  FAKE_HOME="$BATS_TEST_TMPDIR/home"
  export CLAUDE_PLUGIN_ROOT="$FAKE_HOME/.claude"
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
