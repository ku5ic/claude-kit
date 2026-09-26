#!/usr/bin/env bats
# Tests for ~/.dotfiles/claude-kit/hooks/guard-edit.sh.
#
# Each test feeds a synthetic Edit/Write/MultiEdit payload to the hook on
# stdin and asserts the exit code: 0 = allow, 2 = block.
#
# Run with: bats tests/

load helper

setup() {
  HOOK="$BATS_TEST_DIRNAME/../hooks/guard-edit.sh"
}

# Builds an Edit/Write/MultiEdit payload from a path string and pipes it to
# the hook. Uses jq -R so the path can contain any character.
run_guard_edit() {
  hook_payload Edit "$1" | "$HOOK"
}

# positive cases (must allow)

@test "allow: ts source file" {
  run run_guard_edit '/tmp/test.ts'
  [ "$status" -eq 0 ]
}

@test "allow: py source file" {
  run run_guard_edit '/tmp/test.py'
  [ "$status" -eq 0 ]
}

@test "allow: markdown doc" {
  run run_guard_edit '/tmp/foo.md'
  [ "$status" -eq 0 ]
}

@test "allow: nested project file" {
  run run_guard_edit '/tmp/some/nested/dir/file.tsx'
  [ "$status" -eq 0 ]
}

@test "allow: package.json (not a lockfile)" {
  run run_guard_edit '/tmp/package.json'
  [ "$status" -eq 0 ]
}

# negative cases (must block)

# Lockfiles
@test "block: package-lock.json" {
  run run_guard_edit '/tmp/package-lock.json'
  [ "$status" -eq 2 ]
}

@test "block: pnpm-lock.yaml" {
  run run_guard_edit '/tmp/pnpm-lock.yaml'
  [ "$status" -eq 2 ]
}

@test "block: yarn.lock" {
  run run_guard_edit '/tmp/yarn.lock'
  [ "$status" -eq 2 ]
}

@test "block: Gemfile.lock" {
  run run_guard_edit '/tmp/Gemfile.lock'
  [ "$status" -eq 2 ]
}

@test "block: Cargo.lock" {
  run run_guard_edit '/tmp/Cargo.lock'
  [ "$status" -eq 2 ]
}

@test "block: poetry.lock" {
  run run_guard_edit '/tmp/poetry.lock'
  [ "$status" -eq 2 ]
}

@test "block: uv.lock" {
  run run_guard_edit '/tmp/uv.lock'
  [ "$status" -eq 2 ]
}

# .git/ paths
@test "block: edit inside .git/" {
  run run_guard_edit '/tmp/repo/.git/HEAD'
  [ "$status" -eq 2 ]
}

@test "block: edit nested inside .git/" {
  run run_guard_edit '/tmp/repo/.git/refs/heads/main'
  [ "$status" -eq 2 ]
}

# Shell rc files
@test "block: ~/.zshrc" {
  run run_guard_edit "$HOME/.zshrc"
  [ "$status" -eq 2 ]
}

@test "block: ~/.zprofile" {
  run run_guard_edit "$HOME/.zprofile"
  [ "$status" -eq 2 ]
}

@test "block: ~/.bashrc" {
  run run_guard_edit "$HOME/.bashrc"
  [ "$status" -eq 2 ]
}

# Guarded lockfiles come from kit.yml's package_managers and extra_lockfiles.

@test "block: bun.lock (a package_managers lockfile the old list missed)" {
  run run_guard_edit '/tmp/project/bun.lock'
  [ "$status" -eq 2 ]
}

@test "block: Pipfile.lock" {
  run run_guard_edit '/tmp/project/Pipfile.lock'
  [ "$status" -eq 2 ]
}

@test "block: Cargo.lock (extra_lockfiles)" {
  run run_guard_edit '/tmp/project/Cargo.lock'
  [ "$status" -eq 2 ]
}

@test "allow: requirements.txt is hand-edited" {
  run run_guard_edit '/tmp/project/requirements.txt'
  [ "$status" -eq 0 ]
}

# The kit overlay: a write gets a prompt through either path to it.

# Fake HOME whose overlay link points at a file in a fake dotfiles tree.
setup_overlay() {
  kit_test_home
  OVERLAY_SRC="$BATS_TEST_TMPDIR/dotfiles/claude/claude-kit.local.yml"
  mkdir -p "${OVERLAY_SRC%/*}"
  touch "$OVERLAY_SRC"
  ln -s "$OVERLAY_SRC" "$FAKE_HOME/.claude/claude-kit.local.yml"
}

@test "ask: Write to the overlay through its ~/.claude link" {
  setup_overlay
  HOME="$FAKE_HOME" run run_guard_edit "$FAKE_HOME/.claude/claude-kit.local.yml"
  [ "$status" -eq 0 ]
  [[ "$output" == *'"permissionDecision":"ask"'* ]]
}

@test "ask: Write to the overlay's source file in the dotfiles tree" {
  setup_overlay
  HOME="$FAKE_HOME" run run_guard_edit "$OVERLAY_SRC"
  [ "$status" -eq 0 ]
  [[ "$output" == *'"permissionDecision":"ask"'* ]]
}

@test "allow: a same-named file that is not the overlay" {
  setup_overlay
  HOME="$FAKE_HOME" run run_guard_edit "$BATS_TEST_TMPDIR/elsewhere/claude-kit.local.yml"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

