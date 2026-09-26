#!/usr/bin/env bats
# Tests for bin/skills-report.sh.
#
# skills-report.sh reads $HOME/.claude/logs/skills.jsonl and
# $HOME/.claude/kit.yml. Each test fakes $HOME to a fixture dir so real
# machine state never leaks into the assertions.
#
# Run with: bats tests/

load helper

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../bin/skills-report.sh"
  kit_test_home --plugin-root
}

write_log() {
  printf '%s\n' "$@" >"$FAKE_HOME/.claude/logs/skills.jsonl"
}

write_kit_yml() {
  cat >"$FAKE_HOME/.claude/kit.yml"
}

run_report() {
  HOME="$FAKE_HOME" bash "$SCRIPT" "$@"
}

@test "missing log exits 0 with a clear message" {
  run run_report
  [ "$status" -eq 0 ]
  [[ "$output" == *"no log at"* ]]
}

@test "empty log exits 0 with a clear message" {
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"
  run run_report
  [ "$status" -eq 0 ]
  [[ "$output" == *"is empty"* ]]
}

@test "malformed lines are counted and reported, not fatal" {
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  write_log \
    "{\"ts\":\"$ts\",\"hook\":\"log-skills.sh\",\"event\":\"PostToolUse\",\"session_id\":\"s1\",\"cwd\":\"/x\",\"expansion_type\":null,\"command_name\":null,\"command_args\":null,\"command_source\":null,\"skill_file\":\"bash-patterns\",\"tool_name\":\"Skill\"}" \
    'not valid json{{{'
  run run_report
  [ "$status" -eq 0 ]
  [[ "$output" == *"malformed=1"* ]]
  [[ "$output" == *"bash-patterns"* ]]
}

@test "window filter excludes entries older than the requested window" {
  old_ts="$(date -u -v-90d +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '-90 days' +%Y-%m-%dT%H:%M:%SZ)"
  recent_ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  write_log \
    "{\"ts\":\"$old_ts\",\"hook\":\"log-skills.sh\",\"event\":\"PostToolUse\",\"session_id\":\"s1\",\"cwd\":\"/x\",\"expansion_type\":null,\"command_name\":null,\"command_args\":null,\"command_source\":null,\"skill_file\":\"old-skill\",\"tool_name\":\"Skill\"}" \
    "{\"ts\":\"$recent_ts\",\"hook\":\"log-skills.sh\",\"event\":\"PostToolUse\",\"session_id\":\"s1\",\"cwd\":\"/x\",\"expansion_type\":null,\"command_name\":null,\"command_args\":null,\"command_source\":null,\"skill_file\":\"new-skill\",\"tool_name\":\"Skill\"}"

  run run_report 30
  [[ "$output" == *"new-skill"* ]]
  [[ "$output" != *"old-skill"* ]]

  run run_report 120
  [[ "$output" == *"old-skill"* ]]
  [[ "$output" == *"new-skill"* ]]
}

@test "one PostToolUse Skill entry counts as one Skill-tool activation" {
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  write_log \
    "{\"ts\":\"$ts\",\"hook\":\"log-skills.sh\",\"event\":\"PostToolUse\",\"session_id\":\"s1\",\"cwd\":\"/x\",\"expansion_type\":null,\"command_name\":null,\"command_args\":null,\"command_source\":null,\"skill_file\":\"bash-patterns\",\"tool_name\":\"Skill\"}"

  run run_report
  [[ "$output" == *"1  bash-patterns"* ]]
}

@test "required-skill and suggested-skill synthetic entries are reported separately, not counted as activations" {
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  write_log \
    "{\"ts\":\"$ts\",\"hook\":\"inject-context.sh\",\"event\":\"required-skill\",\"session_id\":\"s1\",\"cwd\":\"/x\",\"expansion_type\":null,\"command_name\":null,\"command_args\":null,\"command_source\":null,\"skill_file\":\"fix-sizing\",\"tool_name\":null}" \
    "{\"ts\":\"$ts\",\"hook\":\"inject-context.sh\",\"event\":\"suggested-skill\",\"session_id\":\"s2\",\"cwd\":\"/x\",\"expansion_type\":null,\"command_name\":null,\"command_args\":null,\"command_source\":null,\"skill_file\":\"bash-patterns\",\"tool_name\":null}"

  run run_report
  [[ "$output" == *"required=1"* ]]
  [[ "$output" == *"suggested=1"* ]]
  [[ "$output" == *"(no real activations in the window)"* ]]
}

@test "zero-activation cross-reference against a fixture kit.yml" {
  write_kit_yml <<'YAML'
global_skills:
  - fix-sizing
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
  - on: basename
    globs: ["*.unused"]
    skills: [unused-patterns]
skill_triggers:
  bash-patterns: "before writing shell scripts"
stacks:
  dotfiles:
    skills: [bash-patterns]
YAML

  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  write_log \
    "{\"ts\":\"$ts\",\"hook\":\"log-skills.sh\",\"event\":\"PostToolUse\",\"session_id\":\"s1\",\"cwd\":\"/x\",\"expansion_type\":null,\"command_name\":null,\"command_args\":null,\"command_source\":null,\"skill_file\":\"bash-patterns\",\"tool_name\":\"Skill\"}" \
    "{\"ts\":\"$ts\",\"hook\":\"log-skills.sh\",\"event\":\"UserPromptExpansion\",\"session_id\":\"s1\",\"cwd\":\"/x\",\"expansion_type\":\"slash_command\",\"command_name\":\"/flow-plan\",\"command_args\":null,\"command_source\":\"user\",\"skill_file\":null,\"tool_name\":null}"

  run run_report
  [ "$status" -eq 0 ]
  [[ "$output" == *"== 3: kit.yml-referenced skills with zero activations in the window =="* ]]
  [[ "$output" == *"fix-sizing"* ]]
  [[ "$output" == *"unused-patterns"* ]]
  [[ "$output" == *"== 4: activations for skills not referenced anywhere in kit.yml =="* ]]
  [[ "$output" == *"flow-plan"* ]]
}

@test "sessions with a suggested skill surfaced but never activated are reported" {
  write_kit_yml <<'YAML'
global_skills: []
skill_file_map: []
skill_triggers:
  bash-patterns: "before writing shell scripts"
stacks:
  dotfiles:
    skills: [bash-patterns]
YAML

  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  write_log \
    "{\"ts\":\"$ts\",\"hook\":\"inject-context.sh\",\"event\":\"suggested-skill\",\"session_id\":\"s3\",\"cwd\":\"/x\",\"expansion_type\":null,\"command_name\":null,\"command_args\":null,\"command_source\":null,\"skill_file\":\"bash-patterns\",\"tool_name\":null}"

  run run_report
  [[ "$output" == *"s3: bash-patterns"* ]]
}

@test "guards section counts each rule, its disabled hits, and the last time" {
  local ts
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  write_log "{\"ts\":\"$ts\",\"event\":\"PostToolUse\",\"session_id\":\"s1\",\"skill_file\":\"bash-patterns\",\"tool_name\":\"Skill\"}"
  printf '%s\n' \
    "{\"ts\":\"2026-01-01T00:00:00Z\",\"hook\":\"guard-bash.sh\",\"event\":\"block\",\"rule\":\"rm-recursive\"}" \
    "{\"ts\":\"$ts\",\"hook\":\"guard-bash.sh\",\"event\":\"block\",\"rule\":\"find-delete\"}" \
    "{\"ts\":\"$ts\",\"hook\":\"guard-bash.sh\",\"event\":\"disabled\",\"rule\":\"find-delete\"}" \
    >"$FAKE_HOME/.claude/logs/guards.jsonl"

  run run_report 30
  [ "$status" -eq 0 ]
  [[ "$output" == *"2  find-delete  (disabled=1 last=$ts)"* ]]
  # Outside the 30-day window.
  [[ "$output" != *"rm-recursive"* ]]
}
