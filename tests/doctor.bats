#!/usr/bin/env bats
# Characterization test for bin/doctor.sh: runs it the way CI does and pins
# its exit code and section headers.
#
# It runs on a copy of the files git sees (tracked plus untracked, minus
# ignored) from claude-kit/ and claude/, like a CI checkout would. The live
# tree can hold ignored machine-local content, such as the desktop app's
# skills/synced/, that doctor would flag.
#
# A stub `claude` on PATH lists no MCP servers, so the advisory MCP section
# skips instead of reading the machine's real connector set.

setup() {
  local dotfiles tree="$BATS_TEST_TMPDIR/tree"
  dotfiles="$(cd -P "$BATS_TEST_DIRNAME/../.." && pwd)"
  mkdir -p "$tree"
  git -C "$dotfiles" ls-files -z -co --exclude-standard -- claude-kit claude |
    tar -C "$dotfiles" --null -T - -cf - | tar -C "$tree" -xf -
  SCRIPT="$tree/claude-kit/bin/doctor.sh"
  export HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$HOME"
  unset CLAUDE_PLUGIN_ROOT CLAUDE_CONFIG_DIR
  local stubs="$BATS_TEST_TMPDIR/stubs"
  mkdir -p "$stubs"
  printf '#!/bin/sh\nexit 0\n' >"$stubs/claude"
  chmod +x "$stubs/claude"
  export PATH="$stubs:$PATH"
}

@test "CI run passes and prints every section header in order" {
  CI=true run "$SCRIPT"
  [ "$status" -eq 0 ]
  local headers
  headers="$(printf '%s\n' "$output" | grep '^== ')"
  [ "$headers" = "== prerequisites ==
== symlinks == (skipped: running in CI)
== credential pattern parity ==
== agent-context / inject-context derivation parity ==
== skill + agent frontmatter lint ==
== skill map validation ==
== skills-log field parity ==
== audit-verify field parity ==
== skill directory / allow-list parity ==
== CLAUDE.md rules pointer parity ==
== settings.json machine-local leak ==
== mcp allow-list server parity ==
== plugin hooks.json parity ==" ]
}

@test "missing jq is reported by the prerequisite check, and nothing else runs" {
  # A PATH with bash and yq but no jq; each tool linked by itself, since
  # Homebrew keeps jq in the same directory as bash.
  local bin="$BATS_TEST_TMPDIR/nojq" tool
  mkdir -p "$bin"
  for tool in bash dirname grep yq; do
    ln -s "$(command -v "$tool")" "$bin/$tool"
  done
  CI=true PATH="$bin" run "$bin/bash" "$SCRIPT"
  [ "$status" -eq 1 ]
  [[ "$output" == *"missing        jq (brew install jq)"* ]]
  [[ "$output" != *"== symlinks"* ]]
}
