#!/usr/bin/env bats
# Characterization tests for bin/bootstrap.sh against a fake dotfiles tree
# and a fake HOME. The real script and the lib it sources are copied into the
# fake tree because it locates its sources relative to its own physical path.
#
# A stub `claude` on PATH keeps setup_mcps from touching the real CLI config:
# its `mcp get` succeeds, so every MCP reports ok and nothing is added.

ENTRIES=(settings.json CLAUDE.md hooks skills agents rules bin kit.yml claude-kit.local.yml)

setup() {
  DOT="$BATS_TEST_TMPDIR/dotfiles"
  mkdir -p "$DOT/claude-kit/bin" "$DOT/claude-kit/hooks" "$DOT/claude-kit/skills" "$DOT/claude-kit/agents" "$DOT/claude/rules"
  touch "$DOT/claude-kit/kit.yml" "$DOT/claude/settings.json" "$DOT/claude/CLAUDE.md" "$DOT/claude/claude-kit.local.yml"
  cp "$BATS_TEST_DIRNAME/../bin/bootstrap.sh" "$BATS_TEST_DIRNAME/../bin/_lib.sh" "$DOT/claude-kit/bin/"
  DOT="$(cd -P "$DOT" && pwd)"
  SCRIPT="$DOT/claude-kit/bin/bootstrap.sh"

  export HOME="$BATS_TEST_TMPDIR/home"
  unset CLAUDE_PLUGIN_ROOT CLAUDE_CONFIG_DIR
  mkdir -p "$HOME"

  local stubs="$BATS_TEST_TMPDIR/stubs"
  mkdir -p "$stubs"
  printf '#!/bin/sh\nexit 0\n' >"$stubs/claude"
  chmod +x "$stubs/claude"
  export PATH="$stubs:$PATH"
}

expected_src() {
  case "$1" in
  settings.json | CLAUDE.md | rules | claude-kit.local.yml) echo "$DOT/claude/$1" ;;
  *) echo "$DOT/claude-kit/$1" ;;
  esac
}

@test "fresh HOME: links every entry to its source" {
  run "$SCRIPT" --non-interactive
  [ "$status" -eq 0 ]
  local entry
  for entry in "${ENTRIES[@]}"; do
    [ -L "$HOME/.claude/$entry" ]
    [ "$(readlink "$HOME/.claude/$entry")" = "$(expected_src "$entry")" ]
    [[ "$output" == *"created  $HOME/.claude/$entry"* ]]
  done
  [[ "$output" == *"ok       mcp:context7"* ]]
}

@test "second run reports every entry ok" {
  "$SCRIPT" --non-interactive >/dev/null
  run "$SCRIPT" --non-interactive
  [ "$status" -eq 0 ]
  local entry
  for entry in "${ENTRIES[@]}"; do
    [[ "$output" == *"ok       $HOME/.claude/$entry"* ]]
  done
  [[ "$output" != *"created  $HOME"* ]]
}

@test "non-interactive run refuses to clobber a regular file" {
  mkdir -p "$HOME/.claude"
  printf 'mine\n' >"$HOME/.claude/CLAUDE.md"

  run "$SCRIPT" --non-interactive
  [ "$status" -eq 1 ]
  [[ "$output" == *"refusing to overwrite regular file in non-interactive mode: $HOME/.claude/CLAUDE.md"* ]]
  [ ! -L "$HOME/.claude/CLAUDE.md" ]
  [ "$(cat "$HOME/.claude/CLAUDE.md")" = "mine" ]
  # The other entries are still linked.
  [ -L "$HOME/.claude/settings.json" ]
}

@test "a regular directory is never removed, even with --force" {
  mkdir -p "$HOME/.claude/skills"
  touch "$HOME/.claude/skills/keep"

  run "$SCRIPT" --force
  [ "$status" -eq 1 ]
  [[ "$output" == *"not removing non-symlink directory"* ]]
  [ -e "$HOME/.claude/skills/keep" ]
}

@test "a leftover _stacks.yml link into the kit is removed" {
  mkdir -p "$HOME/.claude"
  ln -s "$DOT/claude-kit/_stacks.yml" "$HOME/.claude/_stacks.yml"

  run "$SCRIPT" --non-interactive
  [ "$status" -eq 0 ]
  [ ! -L "$HOME/.claude/_stacks.yml" ]
  [[ "$output" == *"removed  $HOME/.claude/_stacks.yml"* ]]
}

@test "a _stacks.yml that is not a link into the kit is left alone" {
  mkdir -p "$HOME/.claude"
  printf 'mine\n' >"$HOME/.claude/_stacks.yml"
  ln -s /elsewhere/_stacks.yml "$HOME/.claude/other"

  run "$SCRIPT" --non-interactive
  [ "$status" -eq 0 ]
  [ "$(cat "$HOME/.claude/_stacks.yml")" = "mine" ]
}

@test "a _stacks.yml link pointing outside the kit is left alone" {
  mkdir -p "$HOME/.claude"
  ln -s /elsewhere/_stacks.yml "$HOME/.claude/_stacks.yml"

  run "$SCRIPT" --non-interactive
  [ "$status" -eq 0 ]
  [ "$(readlink "$HOME/.claude/_stacks.yml")" = "/elsewhere/_stacks.yml" ]
}

@test "a symlink pointing elsewhere is refused in non-interactive mode" {
  mkdir -p "$HOME/.claude"
  ln -s /nonexistent "$HOME/.claude/bin"

  run "$SCRIPT" --non-interactive
  [ "$status" -eq 1 ]
  [[ "$output" == *"refusing to replace symlink in non-interactive mode"* ]]
  [ "$(readlink "$HOME/.claude/bin")" = "/nonexistent" ]
}
