#!/usr/bin/env bats
# Tests for ~/.claude/hooks/guard-dispatch.sh.
#
# guard-dispatch.sh is the single PreToolUse hook wired to Edit|Write|
# MultiEdit in settings.json. It sources guard-edit.sh and guard-skills.sh
# once each and runs their run_guard_* functions in declared order
# (edit-safety, then skills-gate), each inside its own subshell with a fresh
# set -e/errtrace/ERR trap so one check's fail-open never masks a different
# check's genuine violation.
#
# Each test fakes $HOME so guard-skills.sh's kit.yml/skills.jsonl/cache
# reads never touch real machine state, same convention as guard-skills.bats.
#
# Run with: bats tests/

load helper

setup() {
  HOOK="$BATS_TEST_DIRNAME/../hooks/guard-dispatch.sh"
  kit_test_home --plugin-root
  export CLAUDE_GUARD_SKILLS=1
}

# run_dispatch <path> <content> [session_id] [tool_name]
# Builds a Write-shaped payload (tool_input.content) with the fields every
# one of the three sourced checks reads, and pipes it to the dispatcher.
run_dispatch() {
  local path="$1" content="$2" session="${3:-s1}" tool_name="${4:-Write}"
  jq -n --arg path "$path" --arg content "$content" --arg sess "$session" --arg tn "$tool_name" \
    '{tool_input: {file_path: $path, content: $content}, session_id: $sess, tool_name: $tn}' |
    HOME="$FAKE_HOME" "$HOOK"
}

@test "clean write with no kit.yml and no risky path passes both checks" {
  run run_dispatch '/tmp/project/notes.md' 'A normal sentence with nothing wrong.'
  [ "$status" -eq 0 ]
}

@test "guard-edit's check fires first and blocks a lockfile edit" {
  # The guarded lockfile list comes from kit.yml.
  cat >"$FAKE_HOME/.claude/kit.yml" <<'YAML'
extra_lockfiles: [package-lock.json]
YAML
  run run_dispatch '/tmp/project/package-lock.json' 'harmless content'
  [ "$status" -eq 2 ]
  [[ "$output" == *"Blocked by guard-edit.sh"* ]]
}

@test "guard-skills' check blocks when a required skill has not been loaded" {
  cat >"$FAKE_HOME/.claude/kit.yml" <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
YAML
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"
  run run_dispatch '/tmp/project/deploy.sh' 'harmless content'
  [ "$status" -eq 2 ]
  [[ "$output" == *"bash-patterns"* ]]
}

@test "guard-skills' check is skipped unless CLAUDE_GUARD_SKILLS=1" {
  cat >"$FAKE_HOME/.claude/kit.yml" <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
YAML
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"
  CLAUDE_GUARD_SKILLS=0 run run_dispatch '/tmp/project/deploy.sh' 'harmless content'
  [ "$status" -eq 0 ]
}

@test "ordering: a lockfile edit that would also trip the skills-gate surfaces only guard-edit's message" {
  cat >"$FAKE_HOME/.claude/kit.yml" <<'YAML'
extra_lockfiles: [yarn.lock]
skill_file_map:
  - on: basename
    globs: ["*.lock"]
    skills: [bash-patterns]
YAML
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"
  run run_dispatch '/tmp/project/yarn.lock' 'harmless content'
  [ "$status" -eq 2 ]
  [[ "$output" == *"Blocked by guard-edit.sh"* ]]
  # The dispatcher exits at the first blocking check instead of running the
  # remaining one, so the skills-gate message never appears alongside it.
  [[ "$output" != *"bash-patterns"* ]]
}

@test "a required skill already loaded this session allows a clean write through both checks" {
  cat >"$FAKE_HOME/.claude/kit.yml" <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
YAML
  jq -nc --arg sid s1 '{ts:"2026-01-01T00:00:00Z",hook:"log-skills.sh",event:"PreToolUse",session_id:$sid,cwd:"/x",expansion_type:null,command_name:null,command_args:null,command_source:null,skill_file:"bash-patterns",tool_name:"Skill"}' \
    >"$FAKE_HOME/.claude/logs/skills.jsonl"
  run run_dispatch '/tmp/project/deploy.sh' 'A perfectly ordinary comment.'
  [ "$status" -eq 0 ]
}

# Resilience: each check runs inside its own subshell with a fresh ERR trap,
# so an unexpected internal error in one does not abort the dispatcher before
# the remaining checks run. Malformed JSON breaks every extract_path/jq call
# identically (both checks share the same $payload), so this cannot
# isolate one specific check's failure from another's success -- but it does
# prove the dispatcher survives and fails open end-to-end instead of hanging
# or erroring out at the first broken check, printing a distinct
# "unexpected error, failing open" notice per check along the way.
@test "malformed JSON payload fails open through both checks instead of erroring out" {
  run bash -c "printf 'not valid json' | HOME='$FAKE_HOME' '$HOOK'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"guard-edit.sh: unexpected error, failing open"* ]]
  [[ "$output" == *"guard-skills.sh: unexpected error, failing open"* ]]
}

@test "empty stdin payload fails open cleanly" {
  run bash -c "printf '' | HOME='$FAKE_HOME' '$HOOK'"
  [ "$status" -eq 0 ]
}

# Fault-injection isolation tests. Copy claude-kit/hooks/*.sh into a scratch dir
# and patch the copy's guard-edit.sh to fail before it does anything real, so
# the implementation under test is never touched. Two fault classes: one the
# subshell's own ERR trap catches (`false`), and one that bypasses it
# entirely (a `set -u` unbound variable - see statusline.sh's own comment on
# the same bypass). Both must leave guard-skills.sh running
# against the untouched implementation.

# bin/ comes along because every hook sources ../bin/_lib.sh.
setup_isolated_hooks() {
  cp -r "$BATS_TEST_DIRNAME/../hooks" "$BATS_TEST_TMPDIR/hooks"
  cp -r "$BATS_TEST_DIRNAME/../bin" "$BATS_TEST_TMPDIR/bin"
  chmod +x "$BATS_TEST_TMPDIR"/hooks/*.sh
}

# inject_fault <fault-command>
# Inserts <fault-command> as the first statement inside the isolated copy's
# guard-edit.sh run_guard_edit(), before it ever reads the payload. awk, not
# sed -i, to stay identical on BSD and GNU without a second code path.
inject_fault() {
  local fault="$1" file="$BATS_TEST_TMPDIR/hooks/guard-edit.sh" tmp
  tmp="$BATS_TEST_TMPDIR/guard-edit-patched.sh"
  awk -v fault="$fault" '
    { print }
    /^  local HOOK_NAME="guard-edit.sh"$/ { print "  " fault }
  ' "$file" >"$tmp"
  mv "$tmp" "$file"
}

# A skills-gate that must block: *.sh requires bash-patterns, nothing loaded.
setup_skills_gate() {
  cat >"$FAKE_HOME/.claude/kit.yml" <<'YAML'
skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
YAML
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"
}

run_dispatch_isolated() {
  local path="$1" content="$2" session="${3:-s1}" tool_name="${4:-Write}"
  jq -n --arg path "$path" --arg content "$content" --arg sess "$session" --arg tn "$tool_name" \
    '{tool_input: {file_path: $path, content: $content}, session_id: $sess, tool_name: $tn}' |
    HOME="$FAKE_HOME" "$BATS_TEST_TMPDIR/hooks/guard-dispatch.sh"
}

@test "isolation survives an ERR-trappable fault in an earlier check" {
  setup_isolated_hooks
  inject_fault 'false'
  setup_skills_gate
  run run_dispatch_isolated '/tmp/project/deploy.sh' 'harmless content'
  [ "$status" -eq 2 ]
  [[ "$output" == *"guard-edit.sh: unexpected error, failing open"* ]]
  [[ "$output" == *"bash-patterns"* ]]
}

@test "isolation survives a fault that bypasses the ERR trap (set -u unbound variable)" {
  setup_isolated_hooks
  inject_fault 'echo "$definitely_not_set"'
  setup_skills_gate
  run run_dispatch_isolated '/tmp/project/deploy.sh' 'harmless content'
  [ "$status" -eq 2 ]
  [[ "$output" == *"bash-patterns"* ]]
}

# Read: guard-edit's credential check applies; the skills gate does not.

# sensitive_paths plus a skill map that gates *.tsx and *.sh edits.
setup_read_guards() {
  cat >"$FAKE_HOME/.claude/kit.yml" <<'YAML'
sensitive_paths: [".env", ".env.*", "~/.ssh/"]
extra_lockfiles: [package-lock.json]
skill_file_map:
  - on: basename
    globs: ["*.tsx", "*.sh"]
    skills: [react-patterns]
YAML
  : >"$FAKE_HOME/.claude/logs/skills.jsonl"
}

@test "Read of .env is blocked" {
  setup_read_guards
  run run_dispatch '/tmp/project/.env' '' s1 Read
  [ "$status" -eq 2 ]
  [[ "$output" == *"reading a credential or key file"* ]]
}

@test "Read of .env.local is blocked" {
  setup_read_guards
  run run_dispatch '/tmp/project/.env.local' '' s1 Read
  [ "$status" -eq 2 ]
}

@test "Read under ~/.ssh is blocked" {
  setup_read_guards
  run run_dispatch "$FAKE_HOME/.ssh/id_ed25519" '' s1 Read
  [ "$status" -eq 2 ]
}

@test "Write of .env is blocked" {
  setup_read_guards
  run run_dispatch '/tmp/project/.env' 'KEY=1' s1 Write
  [ "$status" -eq 2 ]
  [[ "$output" == *"writing a credential or key file"* ]]
}

@test "Read of a .tsx passes with the skills gate on and no skills loaded" {
  setup_read_guards
  run run_dispatch '/tmp/project/App.tsx' '' s1 Read
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "the same .tsx is still gated for an Edit" {
  setup_read_guards
  run run_dispatch '/tmp/project/App.tsx' 'x' s1 Edit
  [ "$status" -eq 2 ]
  [[ "$output" == *"react-patterns"* ]]
}

@test "Read of a lockfile passes; the lockfile check guards writes only" {
  setup_read_guards
  run run_dispatch '/tmp/project/package-lock.json' '' s1 Read
  [ "$status" -eq 0 ]
}

@test "disabling sensitive-read lets the Read through" {
  setup_read_guards
  printf 'disabled_rules: [sensitive-read]\n' >"$FAKE_HOME/.claude/claude-kit.local.yml"
  run run_dispatch '/tmp/project/.env' '' s1 Read
  [ "$status" -eq 0 ]
}
