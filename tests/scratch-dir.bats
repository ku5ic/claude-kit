#!/usr/bin/env bats
# Characterization tests for bin/scratch-dir.sh and bin/plans-dir.sh: the
# project tier inside a git repo, the home fallback outside one.

setup() {
  BIN="$BATS_TEST_DIRNAME/../bin"
  export HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$HOME"
  unset CLAUDE_PLUGIN_ROOT CLAUDE_CONFIG_DIR
  REPO="$BATS_TEST_TMPDIR/repo"
  git init -q -b main "$REPO"
  REPO="$(cd -P "$REPO" && pwd)"
  mkdir -p "$REPO/src"
  # Outside any repo and with no sentinel within three levels.
  OUTSIDE="$BATS_TEST_TMPDIR/plain/a/b"
  mkdir -p "$OUTSIDE"
}

@test "scratch-dir.sh inside a repo prints and creates the project tier" {
  cd "$REPO/src"
  run "$BIN/scratch-dir.sh"
  [ "$status" -eq 0 ]
  [ "$output" = "$REPO/.claude/scratch" ]
  [ -d "$REPO/.claude/scratch" ]
}

@test "scratch-dir.sh <kind> <slug> prints a timestamped report path" {
  cd "$REPO"
  run "$BIN/scratch-dir.sh" perf checkout-page
  [ "$status" -eq 0 ]
  [[ "$output" =~ ^$REPO/\.claude/scratch/perf-checkout-page-[0-9]{8}-[0-9]{4}\.md$ ]]
}

@test "scratch-dir.sh <kind> alone leaves the slug out" {
  cd "$REPO"
  run "$BIN/scratch-dir.sh" deps
  [[ "$output" =~ /deps-[0-9]{8}-[0-9]{4}\.md$ ]]
}

@test "scratch-dir.sh makes the slug filename-safe" {
  cd "$REPO"
  run "$BIN/scratch-dir.sh" review "feat/login page"
  [[ "$output" =~ /review-feat-login-page-[0-9]{8}-[0-9]{4}\.md$ ]]
}

@test "scratch-dir.sh registers the project tier once" {
  cd "$REPO"
  "$BIN/scratch-dir.sh" >/dev/null
  "$BIN/scratch-dir.sh" >/dev/null
  run cat "$HOME/.claude/logs/scratch-registry.txt"
  [ "$output" = "$REPO/.claude/scratch" ]
}

@test "scratch-dir.sh outside a project falls back to home" {
  cd "$OUTSIDE"
  run "$BIN/scratch-dir.sh"
  [ "$status" -eq 0 ]
  [ "$output" = "$HOME/.claude/scratch" ]
  [ -d "$HOME/.claude/scratch" ]
  [ ! -e "$HOME/.claude/logs/scratch-registry.txt" ]
}

@test "plans-dir.sh inside a repo prints and creates the project tier" {
  cd "$REPO/src"
  run "$BIN/plans-dir.sh"
  [ "$status" -eq 0 ]
  [ "$output" = "$REPO/.claude/plans" ]
  [ -d "$REPO/.claude/plans" ]
}

@test "plans-dir.sh registers nothing" {
  cd "$REPO"
  "$BIN/plans-dir.sh" >/dev/null
  [ ! -e "$HOME/.claude/logs/scratch-registry.txt" ]
}

@test "CLAUDE_CONFIG_DIR relocates the fallback and the registry" {
  export CLAUDE_CONFIG_DIR="$BATS_TEST_TMPDIR/config"
  cd "$OUTSIDE"
  run "$BIN/scratch-dir.sh"
  [ "$output" = "$CLAUDE_CONFIG_DIR/scratch" ]

  cd "$REPO"
  "$BIN/scratch-dir.sh" >/dev/null
  [ "$(cat "$CLAUDE_CONFIG_DIR/logs/scratch-registry.txt")" = "$REPO/.claude/scratch" ]
  [ ! -e "$HOME/.claude/logs/scratch-registry.txt" ]
}

@test "plans-dir.sh outside a project falls back to home" {
  cd "$OUTSIDE"
  run "$BIN/plans-dir.sh"
  [ "$status" -eq 0 ]
  [ "$output" = "$HOME/.claude/plans" ]
}
