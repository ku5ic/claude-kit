# Shared setup for the kit's bats tests. Load it with `load helper`.

# kit_test_home [--plugin-root]
# Points FAKE_HOME at a fresh fake home with .claude/logs, and unsets
# CLAUDE_CONFIG_DIR so a developer's relocated config can't redirect test
# writes. With --plugin-root, CLAUDE_PLUGIN_ROOT is the fake .claude, so the
# kit reads the kit.yml a test writes there; without it, the real kit.yml.
kit_test_home() {
  FAKE_HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$FAKE_HOME/.claude/logs"
  unset CLAUDE_CONFIG_DIR
  if [[ "${1:-}" == --plugin-root ]]; then
    export CLAUDE_PLUGIN_ROOT="$FAKE_HOME/.claude"
  else
    unset CLAUDE_PLUGIN_ROOT
  fi
}

# hook_payload <tool> <command-or-path> [session_id] [cwd]
# A PreToolUse payload: tool_input.command for Bash, tool_input.file_path for
# any other tool. An empty session_id or cwd is left out.
hook_payload() {
  jq -cn --arg tool "$1" --arg target "$2" --arg session "${3:-}" --arg cwd "${4:-}" '
    {tool_name: $tool,
     tool_input: (if $tool == "Bash" then {command: $target} else {file_path: $target} end)}
    + (if $session == "" then {} else {session_id: $session} end)
    + (if $cwd == "" then {} else {cwd: $cwd} end)'
}
