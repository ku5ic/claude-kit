#!/usr/bin/env bats
# Tests for ~/.dotfiles/claude-kit/hooks/guard-edit.sh.
#
# Each test feeds a synthetic Edit/Write/MultiEdit payload to the hook on
# stdin and asserts the exit code: 0 = allow, 2 = block.
#
# Run with: bats tests/

setup() {
  HOOK="$BATS_TEST_DIRNAME/../hooks/guard-edit.sh"
}

# Builds an Edit/Write/MultiEdit payload from a path string and pipes it to
# the hook. Uses jq -R so the path can contain any character.
run_guard_edit() {
  printf '%s' "$1" | jq -R '{tool_input: {file_path: .}}' | "$HOOK"
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

