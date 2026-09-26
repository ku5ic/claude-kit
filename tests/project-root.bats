#!/usr/bin/env bats
# Characterization tests for bin/project-root.sh: git toplevel, the sentinel
# walk, the bare-$PWD fallback, and --check.

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../bin/project-root.sh"
  export HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$HOME"
  unset CLAUDE_PLUGIN_ROOT CLAUDE_CONFIG_DIR
}

@test "prints the git toplevel from a nested subdirectory" {
  git init -q -b main "$BATS_TEST_TMPDIR/repo"
  local root
  root="$(cd -P "$BATS_TEST_TMPDIR/repo" && pwd)"
  mkdir -p "$root/src/app"
  cd "$root/src/app"

  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [ "$output" = "$root" ]
}

@test "--check inside a repo exits 0 and prints nothing" {
  git init -q -b main "$BATS_TEST_TMPDIR/repo"
  mkdir -p "$BATS_TEST_TMPDIR/repo/src"
  cd "$BATS_TEST_TMPDIR/repo/src"

  run "$SCRIPT" --check
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "outside a repo, an anchor sentinel two levels up is the root" {
  mkdir -p "$BATS_TEST_TMPDIR/proj/a/b"
  touch "$BATS_TEST_TMPDIR/proj/package.json"
  cd "$BATS_TEST_TMPDIR/proj/a/b"

  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [ "$output" = "$BATS_TEST_TMPDIR/proj" ]

  run "$SCRIPT" --check
  [ "$status" -eq 0 ]
}

@test "outside a repo with no sentinel, prints \$PWD and --check exits 1" {
  mkdir -p "$BATS_TEST_TMPDIR/plain/a/b"
  cd "$BATS_TEST_TMPDIR/plain/a/b"

  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [ "$output" = "$BATS_TEST_TMPDIR/plain/a/b" ]

  run "$SCRIPT" --check
  [ "$status" -eq 1 ]
  [ -z "$output" ]
}
