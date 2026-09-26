#!/usr/bin/env bats
# Tests for ~/.dotfiles/claude-kit/bin/scratch-rotate.sh.
#
# The script hardcodes its home-fallback target dir as $HOME/.claude/scratch
# and its registry as $HOME/.claude/logs/scratch-registry.txt, so every test
# fakes $HOME to keep it off the real ones. Ages are set with backdate_mtime
# rather than sleeping past a retention window.
#
# Run with: bats tests/

load helper

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../bin/scratch-rotate.sh"
  kit_test_home
  SCRATCH="$FAKE_HOME/.claude/scratch"
  REGISTRY="$FAKE_HOME/.claude/logs/scratch-registry.txt"
  mkdir -p "$SCRATCH"
}

# backdate_mtime <file> <seconds_ago>
backdate_mtime() {
  local file="$1" secs="$2" ts
  ts="$(date -v-"${secs}"S +%Y%m%d%H%M.%S 2>/dev/null || date -d "-${secs} seconds" +%Y%m%d%H%M.%S)"
  touch -t "$ts" "$file"
}

run_rotate() {
  HOME="$FAKE_HOME" "$SCRIPT" "$@"
}

@test "prunes an .md artifact older than the retention window" {
  : >"$SCRATCH/old.md"
  backdate_mtime "$SCRATCH/old.md" $((40 * 86400))
  run run_rotate 30
  [ "$status" -eq 0 ]
  [[ "$output" == *"pruned 1 artifact(s)"* ]]
  [ ! -e "$SCRATCH/old.md" ]
}

@test "keeps an .md artifact newer than the retention window" {
  : >"$SCRATCH/fresh.md"
  backdate_mtime "$SCRATCH/fresh.md" 3600
  run run_rotate 30
  [ "$status" -eq 0 ]
  [[ "$output" == *"pruned 0 artifact(s)"* ]]
  [ -e "$SCRATCH/fresh.md" ]
}

@test "prunes a .injected- session marker older than 1 day" {
  : >"$SCRATCH/.injected-old"
  backdate_mtime "$SCRATCH/.injected-old" $((2 * 86400))
  run run_rotate
  [ "$status" -eq 0 ]
  [[ "$output" == *"pruned 1 session marker(s) older than 1d"* ]]
  [ ! -e "$SCRATCH/.injected-old" ]
}

@test "keeps a .injected- session marker younger than 1 day" {
  : >"$SCRATCH/.injected-fresh"
  backdate_mtime "$SCRATCH/.injected-fresh" 3600
  run run_rotate
  [ "$status" -eq 0 ]
  [[ "$output" == *"pruned 0 session marker(s) older than 1d"* ]]
  [ -e "$SCRATCH/.injected-fresh" ]
}

@test "marker retention is fixed at 1 day, independent of the .md days argument" {
  # A 2-day-old marker is pruned even when the .md retention window passed as
  # an argument is generous (100 days) - the two sweeps use unrelated cutoffs.
  : >"$SCRATCH/.injected-old"
  backdate_mtime "$SCRATCH/.injected-old" $((2 * 86400))
  run run_rotate 100
  [ "$status" -eq 0 ]
  [ ! -e "$SCRATCH/.injected-old" ]
}

@test "marker sweep does not descend into subdirectories (maxdepth 1)" {
  mkdir -p "$SCRATCH/sub"
  : >"$SCRATCH/sub/.injected-nested"
  backdate_mtime "$SCRATCH/sub/.injected-nested" $((3 * 86400))
  run run_rotate
  [ "$status" -eq 0 ]
  [ -e "$SCRATCH/sub/.injected-nested" ]
}

@test "both sweeps run together and report independent counts" {
  : >"$SCRATCH/old.md"
  backdate_mtime "$SCRATCH/old.md" $((40 * 86400))
  : >"$SCRATCH/fresh.md"
  backdate_mtime "$SCRATCH/fresh.md" 3600
  : >"$SCRATCH/.injected-old"
  backdate_mtime "$SCRATCH/.injected-old" $((2 * 86400))
  : >"$SCRATCH/.injected-fresh"
  backdate_mtime "$SCRATCH/.injected-fresh" 3600
  run run_rotate 30
  [ "$status" -eq 0 ]
  [[ "$output" == *"pruned 1 artifact(s) older than 30d"* ]]
  [[ "$output" == *"pruned 1 session marker(s) older than 1d"* ]]
  [ ! -e "$SCRATCH/old.md" ]
  [ -e "$SCRATCH/fresh.md" ]
  [ ! -e "$SCRATCH/.injected-old" ]
  [ -e "$SCRATCH/.injected-fresh" ]
}

@test "no scratch dir: exits 0 without error" {
  rm -rf "$SCRATCH"
  run run_rotate
  [ "$status" -eq 0 ]
}

@test "prunes an old file in a registered project scratch dir" {
  PROJ="$FAKE_HOME/proj/scratch"
  mkdir -p "$PROJ"
  : >"$PROJ/poc.py"
  backdate_mtime "$PROJ/poc.py" $((40 * 86400))
  mkdir -p "$(dirname "$REGISTRY")"
  echo "$PROJ" >"$REGISTRY"
  run run_rotate 30
  [ "$status" -eq 0 ]
  [[ "$output" == *"pruned 1 artifact(s) older than 30d from $PROJ"* ]]
  [ ! -e "$PROJ/poc.py" ]
  [[ "$(cat "$REGISTRY")" == "$PROJ" ]]
}

@test "keeps a fresh file in a registered project scratch dir" {
  PROJ="$FAKE_HOME/proj/scratch"
  mkdir -p "$PROJ"
  : >"$PROJ/poc.py"
  backdate_mtime "$PROJ/poc.py" 3600
  mkdir -p "$(dirname "$REGISTRY")"
  echo "$PROJ" >"$REGISTRY"
  run run_rotate 30
  [ "$status" -eq 0 ]
  [[ "$output" == *"pruned 0 artifact(s) older than 30d from $PROJ"* ]]
  [ -e "$PROJ/poc.py" ]
}

@test "drops a registry entry whose project scratch dir no longer exists" {
  PROJ="$BATS_TEST_TMPDIR/proj/scratch"
  mkdir -p "$(dirname "$REGISTRY")"
  echo "$PROJ" >"$REGISTRY"
  run run_rotate
  [ "$status" -eq 0 ]
  [[ "$output" == *"dropping stale registry entry $PROJ"* ]]
  [ ! -s "$REGISTRY" ]
}

@test "prunes multiple registered project dirs and keeps existing entries" {
  PROJ_A="$FAKE_HOME/proj-a/scratch"
  PROJ_B="$FAKE_HOME/proj-b/scratch"
  mkdir -p "$PROJ_A" "$PROJ_B"
  : >"$PROJ_A/old.py"
  backdate_mtime "$PROJ_A/old.py" $((40 * 86400))
  : >"$PROJ_B/fresh.py"
  backdate_mtime "$PROJ_B/fresh.py" 3600
  mkdir -p "$(dirname "$REGISTRY")"
  printf '%s\n%s\n' "$PROJ_A" "$PROJ_B" >"$REGISTRY"
  run run_rotate 30
  [ "$status" -eq 0 ]
  [ ! -e "$PROJ_A/old.py" ]
  [ -e "$PROJ_B/fresh.py" ]
  [[ "$(cat "$REGISTRY")" == "$(printf '%s\n%s' "$PROJ_A" "$PROJ_B")" ]]
}

# The guard that makes every project dir above live under $FAKE_HOME: a
# registry line is untrusted input, so anything outside $HOME is refused
# rather than pruned.
@test "refuses a registered dir outside HOME instead of pruning it" {
  OUTSIDE="$BATS_TEST_TMPDIR/outside/scratch"
  mkdir -p "$OUTSIDE"
  : >"$OUTSIDE/old.py"
  backdate_mtime "$OUTSIDE/old.py" $((40 * 86400))
  mkdir -p "$(dirname "$REGISTRY")"
  echo "$OUTSIDE" >"$REGISTRY"
  run run_rotate 30
  [ "$status" -eq 0 ]
  [[ "$output" == *"REFUSING registry entry $OUTSIDE"* ]]
  [ -e "$OUTSIDE/old.py" ]
  [[ "$(cat "$REGISTRY")" == "$OUTSIDE" ]]
}

@test "no registry file: exits 0 without error" {
  rm -f "$REGISTRY"
  run run_rotate
  [ "$status" -eq 0 ]
}

# guard-skills.sh's per-skill marker cache
# ($HOME/.claude/cache/skills-loaded/<session>-<skill>)

@test "prunes a skills-loaded marker older than 1 day" {
  CACHE="$FAKE_HOME/.claude/cache/skills-loaded"
  mkdir -p "$CACHE"
  : >"$CACHE/s1-bash-patterns"
  backdate_mtime "$CACHE/s1-bash-patterns" $((2 * 86400))
  run run_rotate
  [ "$status" -eq 0 ]
  [[ "$output" == *"pruned 1 skill-loaded marker(s) older than 1d"* ]]
  [ ! -e "$CACHE/s1-bash-patterns" ]
}

@test "keeps a skills-loaded marker younger than 1 day" {
  CACHE="$FAKE_HOME/.claude/cache/skills-loaded"
  mkdir -p "$CACHE"
  : >"$CACHE/s1-bash-patterns"
  backdate_mtime "$CACHE/s1-bash-patterns" 3600
  run run_rotate
  [ "$status" -eq 0 ]
  [[ "$output" == *"pruned 0 skill-loaded marker(s) older than 1d"* ]]
  [ -e "$CACHE/s1-bash-patterns" ]
}

@test "prunes only the stale markers, keeping fresh ones in the same cache dir" {
  CACHE="$FAKE_HOME/.claude/cache/skills-loaded"
  mkdir -p "$CACHE"
  : >"$CACHE/s1-bash-patterns"
  backdate_mtime "$CACHE/s1-bash-patterns" $((2 * 86400))
  : >"$CACHE/s1-typescript-patterns"
  backdate_mtime "$CACHE/s1-typescript-patterns" 3600
  run run_rotate
  [ "$status" -eq 0 ]
  [[ "$output" == *"pruned 1 skill-loaded marker(s) older than 1d"* ]]
  [ ! -e "$CACHE/s1-bash-patterns" ]
  [ -e "$CACHE/s1-typescript-patterns" ]
}

@test "no skills-loaded cache dir: exits 0 without error and skips that line" {
  run run_rotate
  [ "$status" -eq 0 ]
  [[ "$output" != *"skill-loaded marker"* ]]
}

@test "trims every JSONL log to log_max_lines from the overlay" {
  local logs="$FAKE_HOME/.claude/logs"
  mkdir -p "$logs"
  printf 'log_max_lines: 2\n' >"$FAKE_HOME/.claude/claude-kit.local.yml"
  printf '{"n":%s}\n' 1 2 3 4 >"$logs/skills.jsonl"
  printf '{"n":%s}\n' 1 2 3 >"$logs/guards.jsonl"

  run run_rotate
  [ "$status" -eq 0 ]
  [[ "$output" == *"trimmed skills.jsonl from 4 to 2 lines"* ]]
  [[ "$output" == *"trimmed guards.jsonl from 3 to 2 lines"* ]]
  [ "$(cat "$logs/guards.jsonl")" = '{"n":2}
{"n":3}' ]
}
