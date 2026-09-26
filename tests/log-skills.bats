#!/usr/bin/env bats
# Tests for ~/.claude/hooks/log-skills.sh.
#
# log-skills.sh appends one JSONL line per skill activation to
# $HOME/.claude/logs/skills.jsonl, for three payload shapes only: a
# PostToolUse Skill-tool call, a PostToolUse Read of a .../skills/*/SKILL.md
# path, and a UserPromptExpansion with expansion_type "slash_command". A
# substring pre-filter on the raw payload (SKILL.md / Skill / slash_command)
# rejects everything else before require_jq is even reached, so a missing
# jq never breaks an unrelated hook invocation.
#
# Each test fakes $HOME so the log never lands in the real
# ~/.claude/logs/skills.jsonl.
#
# Run with: bats tests/

load helper

setup() {
  HOOK="$BATS_TEST_DIRNAME/../hooks/log-skills.sh"
  kit_test_home
  LOG="$FAKE_HOME/.claude/logs/skills.jsonl"
}

run_log_skills() {
  HOME="$FAKE_HOME" "$HOOK"
}

log_line_count() {
  [ -f "$LOG" ] || { echo 0; return; }
  wc -l <"$LOG" | tr -d ' '
}

# loggable shapes: each must append exactly one line

@test "PostToolUse Skill-tool call is logged with skill_file from tool_input.skill" {
  jq -nc --arg sid s1 '{hook_event_name:"PostToolUse", tool_name:"Skill", tool_input:{skill:"bash-patterns"}, session_id:$sid, cwd:"/x"}' |
    run_log_skills
  [ "$(log_line_count)" -eq 1 ]
  [[ "$(cat "$LOG")" == *'"skill_file":"bash-patterns"'* ]]
  [[ "$(cat "$LOG")" == *'"event":"PostToolUse"'* ]]
}

@test "PostToolUse Read of a SKILL.md path is logged with skill_file from the file path" {
  jq -nc --arg sid s1 '{hook_event_name:"PostToolUse", tool_name:"Read", tool_input:{file_path:"/Users/x/.claude/skills/bash-patterns/SKILL.md"}, session_id:$sid, cwd:"/x"}' |
    run_log_skills
  [ "$(log_line_count)" -eq 1 ]
  [[ "$(cat "$LOG")" == *"skills/bash-patterns/SKILL.md"* ]]
}

@test "UserPromptExpansion with expansion_type slash_command is logged" {
  jq -nc --arg sid s1 '{hook_event_name:"UserPromptExpansion", expansion_type:"slash_command", command_name:"flow-test", session_id:$sid, cwd:"/x"}' |
    run_log_skills
  [ "$(log_line_count)" -eq 1 ]
  [[ "$(cat "$LOG")" == *'"command_name":"flow-test"'* ]]
}

# non-loggable shapes: no log entry

@test "an ordinary Read of a non-SKILL.md file logs nothing" {
  jq -nc '{hook_event_name:"PostToolUse", tool_name:"Read", tool_input:{file_path:"/tmp/project/foo.ts"}, session_id:"s1", cwd:"/x"}' |
    run_log_skills
  [ "$(log_line_count)" -eq 0 ]
}

@test "a PostToolUse call for an unrelated tool logs nothing" {
  jq -nc '{hook_event_name:"PostToolUse", tool_name:"Bash", tool_input:{command:"ls -la"}, session_id:"s1", cwd:"/x"}' |
    run_log_skills
  [ "$(log_line_count)" -eq 0 ]
}

@test "UserPromptExpansion with a non-slash_command expansion_type logs nothing" {
  jq -nc '{hook_event_name:"UserPromptExpansion", expansion_type:"keyword", command_name:"foo", session_id:"s1", cwd:"/x"}' |
    run_log_skills
  [ "$(log_line_count)" -eq 0 ]
}

@test "an unrelated hook_event_name (PreToolUse) logs nothing" {
  jq -nc '{hook_event_name:"PreToolUse", tool_name:"Edit", tool_input:{file_path:"/tmp/foo.md"}, session_id:"s1", cwd:"/x"}' |
    run_log_skills
  [ "$(log_line_count)" -eq 0 ]
}

# pre-filter is intentionally over-inclusive: correctness lives downstream

@test "a payload that incidentally contains the substring 'Skill' but is not a loggable shape still logs nothing" {
  # PreToolUse Bash call whose command text happens to contain the word
  # 'Skill' -- passes the substring pre-filter but the event-type case
  # below still rejects it, proving the pre-filter is a fast-reject
  # optimization, not a replacement for the real routing logic.
  jq -nc '{hook_event_name:"PreToolUse", tool_name:"Bash", tool_input:{command:"echo loading a Skill now"}, session_id:"s1", cwd:"/x"}' |
    run_log_skills
  [ "$(log_line_count)" -eq 0 ]
}

@test "a slash-command-shaped command_name passes the pre-filter but a non-slash_command expansion_type is still rejected downstream" {
  jq -nc '{hook_event_name:"UserPromptExpansion", expansion_type:"keyword", command_name:"/some-Skill-name", session_id:"s1", cwd:"/x"}' |
    run_log_skills
  [ "$(log_line_count)" -eq 0 ]
}

# pre-filter short-circuits before require_jq

# stub_path_no_jq builds a minimal PATH with `cat` and `dirname` (both of
# which the hook needs before it ever reaches the jq check) but no `jq`,
# rather than an empty PATH which would also break the hook's own plumbing.
stub_path_no_jq() {
  local stub_dir="$BATS_TEST_TMPDIR/stub_no_jq"
  mkdir -p "$stub_dir"
  ln -sf "$(command -v cat)" "$stub_dir/cat"
  ln -sf "$(command -v dirname)" "$stub_dir/dirname"
  printf '%s' "$stub_dir"
}

@test "a payload with none of the three substrings exits clean even without jq on PATH" {
  local stub_dir bash_bin
  stub_dir="$(stub_path_no_jq)"
  bash_bin="$(command -v bash)"
  run env PATH="$stub_dir" HOME="$FAKE_HOME" "$bash_bin" "$HOOK" <<<'{"hook_event_name":"PostToolUse","tool_name":"Read","tool_input":{"file_path":"/tmp/foo.ts"}}'
  [ "$status" -eq 0 ]
  [[ "$output" != *"jq not found"* ]]
  [ "$(log_line_count)" -eq 0 ]
}

@test "a loggable payload without jq on PATH fails open with the jq-not-found notice" {
  local stub_dir bash_bin
  stub_dir="$(stub_path_no_jq)"
  bash_bin="$(command -v bash)"
  run env PATH="$stub_dir" HOME="$FAKE_HOME" "$bash_bin" "$HOOK" <<<'{"hook_event_name":"PostToolUse","tool_name":"Skill","tool_input":{"skill":"bash-patterns"}}'
  [ "$status" -eq 0 ]
  [[ "$output" == *"jq not found"* ]]
  [ "$(log_line_count)" -eq 0 ]
}
