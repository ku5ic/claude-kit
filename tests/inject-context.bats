#!/usr/bin/env bats
# Tests for ~/.dotfiles/claude-kit/hooks/inject-context.sh.
#
# inject-context.sh calls project-name.sh/project-root.sh via absolute
# $HOME-prefixed paths, so each test fakes $HOME with tiny stand-in scripts
# for those two collaborators (echoing a fixed name/root) rather than
# exercising the real sentinel walk -- this isolates inject-context.sh's own
# logic, which is what this suite covers. The real hooks/_lib.sh and
# bin/_lib.sh are still sourced (relative to the script's own location), so
# stack-cache and skill-derivation logic is real; only the two collaborator
# scripts and the data files under $HOME are fixtures.
#
# Run with: bats tests/

setup() {
  HOOK="$BATS_TEST_DIRNAME/../hooks/inject-context.sh"
  LIB="$BATS_TEST_DIRNAME/../bin/_lib.sh"
  FAKE_HOME="$BATS_TEST_TMPDIR/home"
  export CLAUDE_PLUGIN_ROOT="$FAKE_HOME/.claude"
  FAKE_ROOT="$BATS_TEST_TMPDIR/project"
  mkdir -p "$FAKE_HOME/.claude/bin" "$FAKE_HOME/.claude/logs" "$FAKE_HOME/.claude/scratch" "$FAKE_ROOT"
  # Real git init: a non-git root breaks inject-context.sh entirely (see the
  # dedicated RISK test below), which would otherwise block every other test.
  git -C "$FAKE_ROOT" init -q

  set_project_name "testproject"
  set_project_root "$FAKE_ROOT"

  cache_path="$(HOME="$FAKE_HOME" bash -c "source '$LIB' >/dev/null 2>&1; stack_cache_file 'testproject' '$FAKE_ROOT'")"
  mkdir -p "$(dirname "$cache_path")"
}

set_project_name() {
  local name="$1"
  cat >"$FAKE_HOME/.claude/bin/project-name.sh" <<EOF
#!/usr/bin/env bash
echo "$name"
EOF
  chmod +x "$FAKE_HOME/.claude/bin/project-name.sh"
}

set_project_root() {
  local root="$1"
  cat >"$FAKE_HOME/.claude/bin/project-root.sh" <<EOF
#!/usr/bin/env bash
echo "$root"
EOF
  chmod +x "$FAKE_HOME/.claude/bin/project-root.sh"
}

write_stacks_yml() {
  cat >"$FAKE_HOME/.claude/_stacks.yml"
}

write_cache() {
  printf '%s\n' "$@" >"$cache_path"
}

# run_inject_context <session_id> [cwd]
run_inject_context() {
  local session="$1" cwd="${2:-$FAKE_ROOT}"
  jq -n --arg sess "$session" --arg cwd "$cwd" '{session_id: $sess, cwd: $cwd}' |
    HOME="$FAKE_HOME" "$HOOK"
}

@test "<required-skills> contains every global_skills entry" {
  write_stacks_yml <<'YAML'
global_skills:
  - fix-sizing
  - context-gathering
skill_triggers: {}
stacks: {}
YAML
  write_cache "root: $FAKE_ROOT" "js: yes"

  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  [[ "$output" == *"<required-skills>"* ]]
  [[ "$output" == *"fix-sizing"* ]]
  [[ "$output" == *"context-gathering"* ]]
}

@test "<suggested-skills> has one line per detected stack skill with its trigger phrase" {
  write_stacks_yml <<'YAML'
global_skills:
  - fix-sizing
skill_triggers:
  react-patterns: "before building or restructuring React components"
stacks:
  js:
    skills: [javascript-patterns]
    extras:
      - name: react
        dep: react
        skills: [react-patterns]
YAML
  write_cache "root: $FAKE_ROOT" "js: yes (react)"

  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  [[ "$output" == *"<suggested-skills>"* ]]
  [[ "$output" == *"before building or restructuring React components: load react-patterns via the Skill tool"* ]]
  [[ "$output" == *"load javascript-patterns via the Skill tool"* ]]
}

@test "a repo with no sentinel produces no injection" {
  set_project_name "unknown"
  write_stacks_yml <<'YAML'
global_skills:
  - fix-sizing
YAML

  run run_inject_context "s1"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "a suggested skill logs a suggested-skill marker to skills.jsonl" {
  write_stacks_yml <<'YAML'
global_skills: []
skill_triggers:
  react-patterns: "before building or restructuring React components"
stacks:
  js:
    skills: []
    extras:
      - name: react
        dep: react
        skills: [react-patterns]
YAML
  write_cache "root: $FAKE_ROOT" "js: yes (react)"

  run run_inject_context "s1"
  [ "$status" -eq 0 ]

  log_file="$FAKE_HOME/.claude/logs/skills.jsonl"
  [ -f "$log_file" ]
  run jq -s '[.[] | select(.event == "suggested-skill" and .session_id == "s1" and .skill_file == "react-patterns")] | length' "$log_file"
  [ "$output" -eq 1 ]
}

# Regression coverage for a fixed bug: inject-context.sh's dirty-file count
# runs `git -C "$project_root" status --porcelain | wc -l | tr -d ' '`. Under
# pipefail, a non-git project_root used to make that pipeline fail, and the
# fail-open ERR trap in hooks/_lib.sh turned that into a silent early exit --
# no repo-context, no required/suggested skills, no marker touch. Fixed by
# falling back to a "dirty-files: unknown" line instead of aborting.
@test "a non-git project root degrades to dirty-files: unknown instead of failing open" {
  non_git_root="$BATS_TEST_TMPDIR/non-git-project"
  mkdir -p "$non_git_root"
  set_project_root "$non_git_root"
  write_stacks_yml <<'YAML'
global_skills:
  - fix-sizing
YAML
  cache_path="$(HOME="$FAKE_HOME" bash -c "source '$LIB' >/dev/null 2>&1; stack_cache_file 'testproject' '$non_git_root'")"
  mkdir -p "$(dirname "$cache_path")"
  write_cache "root: $non_git_root" "js: yes"

  run run_inject_context "s1" "$non_git_root"
  [ "$status" -eq 0 ]
  [[ "$output" == *"<required-skills>"* ]]
  [[ "$output" == *"fix-sizing"* ]]
  [[ "$output" == *"dirty-files (at session start): unknown"* ]]
}
