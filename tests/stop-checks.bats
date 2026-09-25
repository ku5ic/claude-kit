#!/usr/bin/env bats
# Tests for ~/.dotfiles/claude-kit/hooks/stop-checks.sh.
#
# stop-checks.sh calls run-checks.sh via its absolute $HOME-prefixed path, so
# each test fakes $HOME with a stub that passes or fails on demand. The repo
# is a real git init so the tree-hash logic is exercised for real.
#
# Run with: bats tests/

setup() {
  HOOK="$BATS_TEST_DIRNAME/../hooks/stop-checks.sh"
  FAKE_HOME="$BATS_TEST_TMPDIR/home"
  export CLAUDE_PLUGIN_ROOT="$FAKE_HOME/.claude"
  REPO="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$FAKE_HOME/.claude/bin" "$REPO"
  git -C "$REPO" init -q
  git -C "$REPO" -c user.email=t@t -c user.name=t commit -q --allow-empty -m init
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

stop() {
  local active="${1:-false}"
  jq -nc --arg cwd "$REPO" --argjson active "$active" \
    '{hook_event_name:"Stop", session_id:"s1", cwd:$cwd, stop_hook_active:$active}' |
    HOME="$FAKE_HOME" "$HOOK"
}

@test "clean tree on first stop runs nothing" {
  set_checks fail
  run stop
  [ "$status" -eq 0 ]
}

@test "stop_hook_active lets the stop through even when checks would fail" {
  echo x >"$REPO/a"
  set_checks fail
  run stop true
  [ "$status" -eq 0 ]
}

@test "changed tree with passing checks allows the stop" {
  echo x >"$REPO/a"
  run stop
  [ "$status" -eq 0 ]
}

@test "changed tree with failing checks blocks with the FAIL lines only" {
  echo x >"$REPO/a"
  set_checks fail
  run stop
  [ "$status" -eq 2 ]
  [[ "$output" == *"FAIL lint"* ]]
  [[ "$output" != *"noise"* ]]
}

@test "unchanged tree since the last stop skips the checks" {
  echo x >"$REPO/a"
  run stop
  [ "$status" -eq 0 ]
  set_checks fail
  run stop
  [ "$status" -eq 0 ]
}

@test "a further change after a stop reruns the checks" {
  echo x >"$REPO/a"
  run stop
  echo y >>"$REPO/a"
  set_checks fail
  run stop
  [ "$status" -eq 2 ]
}

@test "an untracked symlink to a directory still runs the checks" {
  mkdir -p "$REPO/target"
  echo x >"$REPO/target/f"
  ln -s target "$REPO/link"
  set_checks fail
  run stop
  [ "$status" -eq 2 ]
  [[ "$output" != *"failing open"* ]]
}

@test "outside a git worktree exits clean" {
  REPO="$BATS_TEST_TMPDIR/plain"
  mkdir -p "$REPO"
  set_checks fail
  run stop
  [ "$status" -eq 0 ]
}
