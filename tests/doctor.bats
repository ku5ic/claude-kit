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
  local kit dir tree="$BATS_TEST_TMPDIR/tree"
  kit="$(cd -P "$BATS_TEST_DIRNAME/.." && pwd)"
  # The personal claude/ sits beside the kit only in the dotfiles layout.
  for dir in "$kit" "$kit/../claude"; do
    [[ -d "$dir" ]] || continue
    mkdir -p "$tree/${dir##*/}"
    git -C "$dir" ls-files -z -co --exclude-standard -- . |
      tar -C "$dir" --null -T - -cf - | tar -C "$tree/${dir##*/}" -xf -
  done
  mv "$tree/${kit##*/}" "$tree/claude-kit" 2>/dev/null || true
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
  [[ -d "$BATS_TEST_TMPDIR/tree/claude" ]] || skip "no personal claude/ beside the kit"
  CI=true run "$SCRIPT"
  [ "$status" -eq 0 ]
  local headers
  headers="$(printf '%s\n' "$output" | grep '^== ')"
  [ "$headers" = "== prerequisites ==
== symlinks == (skipped: running in CI)
== agent-context / inject-context derivation parity ==
== skill + agent frontmatter lint ==
== skill map validation ==
== skills-log field parity ==
== audit-verify field parity ==
== kit.yml schema ==
== credential pattern parity ==
== skill directory / allow-list parity ==
== CLAUDE.md rules pointer parity ==
== settings.json machine-local leak ==
== mcp allow-list server parity ==
== plugin hooks.json parity ==" ]
}

@test "a standalone kit checkout skips the personal-config sections and passes" {
  rm -rf "$BATS_TEST_TMPDIR/tree/claude"
  CI=true run "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"== credential pattern parity == (skipped: no personal config"* ]]
  [[ "$output" == *"== plugin hooks.json parity == (skipped: no personal config"* ]]
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
