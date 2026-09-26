#!/usr/bin/env bats
# Tests for ~/.dotfiles/claude-kit/hooks/guard-skills.sh.
#
# guard-skills.sh reads $HOME/.claude/kit.yml (skill_file_map) and
# $HOME/.claude/logs/skills.jsonl (what has been loaded this session) on
# every Edit/Write/MultiEdit. Each test fakes $HOME so real machine
# state never leaks into the assertions; some tests copy the real repo's
# kit.yml into the fake $HOME so the production map itself is exercised.
#
# Run with: bats tests/

load helper

setup() {
  HOOK="$BATS_TEST_DIRNAME/../hooks/guard-skills.sh"
  REAL_STACKS_YML="$BATS_TEST_DIRNAME/../kit.yml"
  kit_test_home --plugin-root
}

write_kit_yml() {
  cat >"$FAKE_HOME/.claude/kit.yml"
}

write_skills_log() {
  printf '%s\n' "$@" >"$FAKE_HOME/.claude/logs/skills.jsonl"
}

# run_guard_skills <path> [session_id] [tool_name]
run_guard_skills() {
  hook_payload "${3:-Edit}" "$1" "${2:-s1}" | HOME="$FAKE_HOME" "$HOOK"
}

@test "every skill_file_map entry in the real kit.yml blocks until its skill is loaded" {
  cp "$REAL_STACKS_YML" "$FAKE_HOME/.claude/kit.yml"
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"

  while IFS=$'\t' read -r globs skills; do
    read -ra glob_arr <<<"$globs"
    sample_glob="${glob_arr[0]}"
    filename="${sample_glob//\*/sample}"
    run run_guard_skills "/tmp/project/$filename"
    [ "$status" -eq 2 ]
    for sk in $skills; do
      [[ "$output" == *"$sk"* ]]
    done
  done < <(yq -r '.skill_file_map[] | [(.globs // [] | join(" ")), (.skills // [] | join(" "))] | join("\t")' "$REAL_STACKS_YML")
}

@test "declaration order: a .test.tsx file picks up test-patterns before typescript-patterns" {
  cp "$REAL_STACKS_YML" "$FAKE_HOME/.claude/kit.yml"
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"
  run run_guard_skills "/tmp/project/foo.test.tsx"
  [ "$status" -eq 2 ]
  # Position, not membership: test-patterns must appear earlier in the
  # message than typescript-patterns, matching kit.yml's declared order.
  before_test="${output%%test-patterns*}"
  before_ts="${output%%typescript-patterns*}"
  [ "${#before_test}" -lt "${#before_ts}" ]
}

@test "cumulative matching: a .test.tsx file requires skills from every matching entry, not just one" {
  cp "$REAL_STACKS_YML" "$FAKE_HOME/.claude/kit.yml"
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"
  run run_guard_skills "/tmp/project/foo.test.tsx"
  [ "$status" -eq 2 ]
  [[ "$output" == *"test-patterns"* ]]
  [[ "$output" == *"typescript-patterns"* ]]
  [[ "$output" == *"react-patterns"* ]]
}

# kit.yml's skill_file_map no longer carries a catch-all globs: ["*"] row
# (removed deliberately; fix-sizing/root-cause-diagnosis/context-gathering
# moved to rules/*.md, unconditional and not per-file requirements), so this
# exercises the composing mechanism itself via a synthetic map rather than
# asserting on production data that no longer has a catch-all row.
@test "a catch-all glob entry composes with a specific entry rather than displacing it" {
  write_kit_yml <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*"]
    skills: [fix-sizing]
  - on: basename
    globs: ["*.ts"]
    skills: [typescript-patterns]
YAML
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"
  run run_guard_skills "/tmp/project/foo.ts"
  [ "$status" -eq 2 ]
  [[ "$output" == *"typescript-patterns"* ]]
  [[ "$output" == *"fix-sizing"* ]]
}

@test "on: path entries match the full path, not just the basename" {
  # Synthetic map: the real kit.yml has no on:path entries, but the
  # matching mode is still supported and needs coverage.
  write_kit_yml <<'YAML'
skill_file_map:
  - on: path
    globs: ["*/widgets/*/CONFIG.md"]
    skills: [widget-patterns]
YAML
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"

  run run_guard_skills "/tmp/random/CONFIG.md"
  [[ "$output" != *"widget-patterns"* ]]

  run run_guard_skills "/tmp/project/widgets/foo/CONFIG.md"
  [[ "$output" == *"widget-patterns"* ]]
}

@test "blocks when the required skill has not been loaded this session" {
  write_kit_yml <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
YAML
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"
  run run_guard_skills "/tmp/project/foo.sh"
  [ "$status" -eq 2 ]
  [[ "$output" == *"bash-patterns"* ]]
}

@test "allows when the required skill was loaded this session via the Skill tool" {
  write_kit_yml <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
YAML
  write_skills_log '{"ts":"2026-01-01T00:00:00Z","hook":"log-skills.sh","event":"PreToolUse","session_id":"s1","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"bash-patterns","tool_name":"Skill"}'
  run run_guard_skills "/tmp/project/foo.sh" "s1"
  [ "$status" -eq 0 ]
}

@test "allows when the required skill's SKILL.md was read this session (Read fallback)" {
  write_kit_yml <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
YAML
  write_skills_log '{"ts":"2026-01-01T00:00:00Z","hook":"log-skills.sh","event":"PostToolUse","session_id":"s1","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"/Users/x/.claude/skills/bash-patterns/SKILL.md","tool_name":"Read"}'
  run run_guard_skills "/tmp/project/foo.sh" "s1"
  [ "$status" -eq 0 ]
}

@test "a session_id mismatch does not count as loaded" {
  write_kit_yml <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
YAML
  write_skills_log '{"ts":"2026-01-01T00:00:00Z","hook":"log-skills.sh","event":"PreToolUse","session_id":"other-session","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"bash-patterns","tool_name":"Skill"}'
  run run_guard_skills "/tmp/project/foo.sh" "s1"
  [ "$status" -eq 2 ]
}

@test "Edit tool_name produces an edit-verb block message" {
  write_kit_yml <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
YAML
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"
  run run_guard_skills "/tmp/project/foo.sh" "s1" "Edit"
  [ "$status" -eq 2 ]
  [[ "$output" == *"This edit touches"* ]]
}

# Regression coverage for a fixed enforcement-floor gap (see the plan and
# bin/skills-report.sh Phase 2 for context): a skill can be both
# stack-suggested (inject-context.sh logs a "suggested-skill" marker for
# these, meaning "surfaced", not "loaded") and skill_file_map-required (e.g.
# react-patterns in production). guard-skills.sh's compliance query now
# restricts to real invocation events (PreToolUse/PostToolUse/
# UserPromptExpansion), so a bare "suggested-skill" or "required-skill"
# marker can no longer satisfy a required-skill check on its own.
@test "a suggested-skill marker alone does not satisfy the required-skill check" {
  write_kit_yml <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.jsx"]
    skills: [react-patterns]
YAML
  write_skills_log '{"ts":"2026-01-01T00:00:00Z","hook":"inject-context.sh","event":"suggested-skill","session_id":"s1","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"react-patterns","tool_name":null}'
  run run_guard_skills "/tmp/project/foo.jsx" "s1"
  [ "$status" -eq 2 ]
  [[ "$output" == *"react-patterns"* ]]
}

@test "a required-skill marker alone does not satisfy the required-skill check" {
  write_kit_yml <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*"]
    skills: [fix-sizing]
YAML
  write_skills_log '{"ts":"2026-01-01T00:00:00Z","hook":"inject-context.sh","event":"required-skill","session_id":"s1","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"fix-sizing","tool_name":null}'
  run run_guard_skills "/tmp/project/foo.txt" "s1"
  [ "$status" -eq 2 ]
  [[ "$output" == *"fix-sizing"* ]]
}

# per-skill marker cache ($HOME/.claude/cache/skills-loaded/<session>-<skill>)

@test "an allowed session/skill pair writes a marker file to the cache" {
  write_kit_yml <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
YAML
  write_skills_log '{"ts":"2026-01-01T00:00:00Z","hook":"log-skills.sh","event":"PreToolUse","session_id":"s1","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"bash-patterns","tool_name":"Skill"}'
  run run_guard_skills "/tmp/project/foo.sh" "s1"
  [ "$status" -eq 0 ]
  [ -f "$FAKE_HOME/.claude/cache/skills-loaded/s1-bash-patterns" ]
}

@test "a cached marker allows a second call even when the skills log becomes unreadable" {
  write_kit_yml <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
YAML
  write_skills_log '{"ts":"2026-01-01T00:00:00Z","hook":"log-skills.sh","event":"PreToolUse","session_id":"s1","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"bash-patterns","tool_name":"Skill"}'
  run run_guard_skills "/tmp/project/foo.sh" "s1"
  [ "$status" -eq 0 ]

  # Simulate the log becoming unreadable after the marker was cached; a fresh
  # (uncached) skill for a session with no readable log would fail open
  # per the next test, but this session/skill pair should never need to
  # consult the log again at all.
  chmod 000 "$FAKE_HOME/.claude/logs/skills.jsonl"
  run run_guard_skills "/tmp/project/bar.sh" "s1"
  chmod 644 "$FAKE_HOME/.claude/logs/skills.jsonl"
  [ "$status" -eq 0 ]
}

@test "an uncached skill still fails open when the skills log is unreadable" {
  ((EUID != 0)) || skip "root reads a chmod 000 file, so the log is never unreadable"
  write_kit_yml <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
YAML
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"
  chmod 000 "$FAKE_HOME/.claude/logs/skills.jsonl"
  run run_guard_skills "/tmp/project/foo.sh" "s1"
  chmod 644 "$FAKE_HOME/.claude/logs/skills.jsonl"
  [ "$status" -eq 0 ]
}

@test "a marker for one session does not satisfy a different session's check" {
  write_kit_yml <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
YAML
  write_skills_log '{"ts":"2026-01-01T00:00:00Z","hook":"log-skills.sh","event":"PreToolUse","session_id":"s1","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"bash-patterns","tool_name":"Skill"}'
  run run_guard_skills "/tmp/project/foo.sh" "s1"
  [ "$status" -eq 0 ]
  run run_guard_skills "/tmp/project/foo.sh" "s2"
  [ "$status" -eq 2 ]
}

# kit.yml -> skill_file_map cache ($HOME/.claude/cache/skill-map.base, or
# .merged with an overlay)

@test "the skill-map cache is created after the first invocation" {
  write_kit_yml <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
YAML
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"
  run run_guard_skills "/tmp/project/foo.sh" "s1"
  [ "$status" -eq 2 ]
  [ -s "$FAKE_HOME/.claude/cache/skill-map.base" ]
  [[ "$(cat "$FAKE_HOME/.claude/cache/skill-map.base")" == *"bash-patterns"* ]]
}

@test "a stale skill-map cache (older than kit.yml) is not reused" {
  write_kit_yml <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
YAML
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"
  run run_guard_skills "/tmp/project/foo.sh" "s1"
  [ "$status" -eq 2 ]
  [[ "$output" == *"bash-patterns"* ]]

  # Rewrite kit.yml with a different required skill and make it newer
  # than the cache file just written above.
  write_kit_yml <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [python-patterns]
YAML
  run run_guard_skills "/tmp/project/bar.sh" "s1"
  [ "$status" -eq 2 ]
  [[ "$output" == *"python-patterns"* ]]
  [[ "$output" != *"bash-patterns"* ]]
}

@test "a fresh skill-map cache (newer than kit.yml) is reused instead of re-parsing" {
  write_kit_yml <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
YAML
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"
  run run_guard_skills "/tmp/project/foo.sh" "s1"
  [ "$status" -eq 2 ]
  cache_file="$FAKE_HOME/.claude/cache/skill-map.base"
  [ -s "$cache_file" ]

  # Corrupt the on-disk kit.yml so a fresh parse would produce a
  # different (or no) result, but leave its mtime older than the cache -
  # the cached map should still be what guard-skills.sh reads from.
  printf 'not: [valid, yaml, skill_file_map' >"$FAKE_HOME/.claude/kit.yml"
  touch -t 202001010000 "$FAKE_HOME/.claude/kit.yml"
  touch "$cache_file"

  run run_guard_skills "/tmp/project/bar.sh" "s1"
  [ "$status" -eq 2 ]
  [[ "$output" == *"bash-patterns"* ]]
}
