#!/usr/bin/env bats
# Tests for ~/.dotfiles/claude-kit/bin/statusline.sh.
#
# statusline.sh reads the statusLine JSON payload from stdin and renders a
# two-row status. Each test fakes $HOME so the git-status cache never lands
# in the real ~/.claude/cache, and builds a throwaway git repo for the
# branch/counts assertions so real repo state never leaks in either.
#
# Run with: bats tests/

load helper

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../bin/statusline.sh"
  kit_test_home
  REPO="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$REPO"
  git -C "$REPO" init -q -b main
  git -C "$REPO" config user.email "test@example.com"
  git -C "$REPO" config user.name "Test"
}

# make_payload <dir> <session_id> <ctx_pct> [effort_level] [five_h_pct]
make_payload() {
  local dir="$1" session="$2" ctx="$3" effort="${4:-}" five_h="${5:-}" duration_ms="${6:-1000}"
  jq -n \
    --arg dir "$dir" --arg session "$session" --argjson ctx "$ctx" \
    --arg effort "$effort" --arg five_h "$five_h" --argjson duration_ms "$duration_ms" '
    {
      model: {display_name: "Opus"},
      workspace: {current_dir: $dir},
      session_id: $session,
      context_window: {used_percentage: $ctx},
      cost: {total_cost_usd: 1, total_duration_ms: $duration_ms}
    }
    + (if $effort != "" then {effort: {level: $effort}} else {} end)
    + (if $five_h != "" then {rate_limits: {five_hour: {used_percentage: ($five_h | tonumber)}}} else {} end)
  '
}

run_statusline() {
  printf '%s' "$1" | HOME="$FAKE_HOME" bash "$SCRIPT"
}

# strip_ansi <text>  removes color escape codes so tests can assert on plain
# segment text/ordering without hardcoding which parts statusline.sh colors.
strip_ansi() {
  printf '%s' "$1" | sed -E $'s/\x1b\\[[0-9;]*m//g'
}

# with_agent <payload> <jq-object>  merges agent identity keys into a payload.
# make_payload omits them because a plain interactive session has none; only
# a main thread running as a named agent gets them.
with_agent() {
  jq -c ". + $2" <<<"$1"
}

# with_transcript <payload> <transcript_path>  merges transcript_path into a
# payload. make_payload omits it since most tests never need the
# transcript-derived actual/declared model logic.
with_transcript() {
  jq -c --arg t "$2" '. + {transcript_path: $t}' <<<"$1"
}

# Transcript-line builders for the actual/declared model divergence logic.
# Each takes an ISO-ish timestamp string; lexicographic order must match
# chronological order for the jq `>=` comparisons in statusline.sh to work.
user_line() {
  local text="$1" ts="$2"
  jq -nc --arg text "$text" --arg ts "$ts" \
    '{type: "user", isMeta: false, message: {content: $text}, timestamp: $ts}'
}

# assistant_line <model_id> <ts> [sidechain(true|false)]
assistant_line() {
  local model="$1" ts="$2" sidechain="${3:-false}"
  jq -nc --arg model "$model" --arg ts "$ts" --argjson sidechain "$sidechain" \
    '{type: "assistant", isSidechain: $sidechain, message: {model: $model}, timestamp: $ts}'
}

# declared_line <model_id> <ts>  a command_permissions attachment, the
# recorded signal for a skill's frontmatter model: override at invocation time.
declared_line() {
  local model="$1" ts="$2"
  jq -nc --arg model "$model" --arg ts "$ts" \
    '{type: "attachment", attachment: {type: "command_permissions", model: $model}, timestamp: $ts}'
}

# user_line_array <text> <ts>  same as user_line but with .message.content as
# an array of content blocks - the shape real transcripts overwhelmingly use
# (vs the plain-string shape user_line builds), exercising the $since
# filter's other branch: `[.[]? | select(.type == "text") | .text] | join("\n")`.
user_line_array() {
  local text="$1" ts="$2"
  jq -nc --arg text "$text" --arg ts "$ts" \
    '{type: "user", isMeta: false, message: {content: [{type: "text", text: $text}]}, timestamp: $ts}'
}

# write_transcript <file> <line> [line ...]
write_transcript() {
  local file="$1"
  shift
  printf '%s\n' "$@" >"$file"
}

# backdate_mtime <file> <seconds_ago>
# Sets file's mtime to now minus <seconds_ago>, so TTL-boundary tests can hit
# the exact edge deterministically instead of sleeping past it (which is
# flaky). Tries BSD `date -v` (macOS) first, falls back to GNU `date -d`
# (Linux CI) - the same dual-platform shape statusline.sh itself uses for
# `stat -c` / `stat -f`.
backdate_mtime() {
  local file="$1" secs="$2" ts
  ts="$(date -v-"${secs}"S +%Y%m%d%H%M.%S 2>/dev/null || date -d "-${secs} seconds" +%Y%m%d%H%M.%S)"
  touch -t "$ts" "$file"
}

@test "renders model name, dir basename, and context percentage" {
  run run_statusline "$(make_payload "$REPO" t1 50)"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Opus"* ]]
  [[ "$output" == *"$(basename "$REPO")"* ]]
  [[ "$output" == *"50%"* ]]
}

@test "context bar renders green below the yellow threshold" {
  run run_statusline "$(make_payload "$REPO" t2 50)"
  [[ "$output" == *$'\033[32m'* ]]
}

@test "context bar renders yellow at the yellow threshold" {
  run run_statusline "$(make_payload "$REPO" t3 75)"
  [[ "$output" == *$'\033[33m'* ]]
}

@test "context bar renders red at the red threshold" {
  run run_statusline "$(make_payload "$REPO" t4 95)"
  [[ "$output" == *$'\033[31m'* ]]
}

@test "context bar stays green just below the yellow threshold" {
  run run_statusline "$(make_payload "$REPO" t4a 69)"
  [[ "$output" == *$'\033[32m'* ]]
}

@test "context bar turns yellow exactly at the yellow threshold" {
  run run_statusline "$(make_payload "$REPO" t4b 70)"
  [[ "$output" == *$'\033[33m'* ]]
}

@test "context bar stays yellow just below the red threshold" {
  run run_statusline "$(make_payload "$REPO" t4c 89)"
  [[ "$output" == *$'\033[33m'* ]]
}

@test "context bar turns red exactly at the red threshold" {
  run run_statusline "$(make_payload "$REPO" t4d 90)"
  [[ "$output" == *$'\033[31m'* ]]
}

@test "negative context percentage clamps to zero and renders green" {
  run run_statusline "$(make_payload "$REPO" t4e -15)"
  [[ "$output" == *"0%"* ]]
  [[ "$output" == *$'\033[32m'* ]]
}

@test "context percentage above 100 clamps to 100 and renders red" {
  run run_statusline "$(make_payload "$REPO" t4f 150)"
  [[ "$output" == *"100%"* ]]
  [[ "$output" == *$'\033[31m'* ]]
}

@test "agent name renders beside the model when present" {
  run run_statusline "$(with_agent "$(make_payload "$REPO" t4g 50)" '{agent:{name:"scout"}}')"
  [ "$status" -eq 0 ]
  [[ "$(strip_ansi "$output")" == "Opus (scout)  $(basename "$REPO")"* ]]
}

@test "agent name falls back to agent_type when the agent key is absent" {
  run run_statusline "$(with_agent "$(make_payload "$REPO" t4h 50)" '{agent_type:"reviewer"}')"
  [[ "$(strip_ansi "$output")" == "Opus (reviewer)"* ]]
}

@test "agent key wins over agent_type when both are present" {
  run run_statusline "$(with_agent "$(make_payload "$REPO" t4i 50)" '{agent:{name:"scout"},agent_type:"reviewer"}')"
  [[ "$(strip_ansi "$output")" == "Opus (scout)"* ]]
  [[ "$output" != *"reviewer"* ]]
}

@test "agent segment is omitted for a plain interactive session" {
  run run_statusline "$(make_payload "$REPO" t4j 50)"
  [[ "$(strip_ansi "$output")" == "Opus  $(basename "$REPO")"* ]]
  [[ "$output" != *"("* ]]
}

@test "effort segment renders when present" {
  run run_statusline "$(make_payload "$REPO" t5 50 high)"
  [[ "$output" == *"effort:high"* ]]
}

@test "effort segment is omitted when absent" {
  run run_statusline "$(make_payload "$REPO" t6 50)"
  [[ "$output" != *"effort:"* ]]
}

@test "5h segment renders when present" {
  run run_statusline "$(make_payload "$REPO" t7 50 "" 12.7)"
  [[ "$output" == *"5h:12%"* ]]
}

@test "5h segment is omitted when absent" {
  run run_statusline "$(make_payload "$REPO" t8 50)"
  [[ "$output" != *"5h:"* ]]
}

@test "duration under a minute renders as seconds only" {
  run run_statusline "$(make_payload "$REPO" tdur1 50 "" "" 46000)"
  [[ "$(strip_ansi "$output")" == *" 46s"* ]]
}

@test "duration under an hour renders as minutes only, seconds dropped" {
  run run_statusline "$(make_payload "$REPO" tdur2 50 "" "" 125000)"
  [[ "$(strip_ansi "$output")" == *" 2m" ]]
}

@test "duration of an hour or more rolls into space-separated hours and minutes" {
  run run_statusline "$(make_payload "$REPO" tdur3 50 "" "" 16546000)"
  [[ "$(strip_ansi "$output")" == *" 4h 35m" ]]
}

@test "git segment shows branch and additions/deletions across staged and unstaged changes" {
  printf 'one' >"$REPO/a.txt"
  git -C "$REPO" add a.txt
  git -C "$REPO" commit -qm init
  printf 'two' >"$REPO/b.txt"
  git -C "$REPO" add b.txt
  printf 'changed' >>"$REPO/a.txt"
  run run_statusline "$(make_payload "$REPO" t9 50)"
  [[ "$(strip_ansi "$output")" == *"main +2 ~1"* ]]
}

@test "git segment is omitted outside a git repo" {
  local non_git="$BATS_TEST_TMPDIR/plain"
  mkdir -p "$non_git"
  run run_statusline "$(make_payload "$non_git" t10 50)"
  [ "$(strip_ansi "$output" | head -1)" = "Opus  plain" ]
}

# The default CACHE_TTL is 1 second, which is far too narrow to assert cache
# REUSE against: the git commits between two run_statusline calls can easily
# take longer than the window, so the second call would legitimately refresh
# and the test would flake. Reuse tests therefore export a wide
# STATUSLINE_CACHE_TTL and make no assumption about elapsed wall time;
# expiry tests keep the real 1s default and backdate the cache mtime so the
# boundary is hit deterministically, with no sleep.

@test "git status cache is reused within the TTL" {
  export STATUSLINE_CACHE_TTL=3600
  printf 'one' >"$REPO/a.txt"
  git -C "$REPO" add a.txt
  git -C "$REPO" commit -qm init
  printf 'x' >>"$REPO/a.txt"
  run run_statusline "$(make_payload "$REPO" tcache1 50)"
  [[ "$(strip_ansi "$output")" == *"+1 ~1"* ]]

  printf 'two' >"$REPO/b.txt"
  git -C "$REPO" add b.txt
  git -C "$REPO" commit -qm second
  printf 'y' >>"$REPO/b.txt"
  run run_statusline "$(make_payload "$REPO" tcache1 50)"
  [[ "$(strip_ansi "$output")" == *"+1 ~1"* ]]
}

@test "git status cache regenerates after the TTL expires" {
  printf 'one' >"$REPO/a.txt"
  git -C "$REPO" add a.txt
  git -C "$REPO" commit -qm init
  printf 'x' >>"$REPO/a.txt"
  run run_statusline "$(make_payload "$REPO" tcache2 50)"
  [[ "$(strip_ansi "$output")" == *"+1 ~1"* ]]

  printf 'two' >"$REPO/b.txt"
  git -C "$REPO" add b.txt
  git -C "$REPO" commit -qm second
  printf 'y' >>"$REPO/b.txt"
  sleep 2
  run run_statusline "$(make_payload "$REPO" tcache2 50)"
  [[ "$(strip_ansi "$output")" == *"+2 ~2"* ]]
}

@test "git status cache regenerates exactly at the TTL boundary (age == CACHE_TTL)" {
  printf 'one' >"$REPO/a.txt"
  git -C "$REPO" add a.txt
  git -C "$REPO" commit -qm init
  printf 'x' >>"$REPO/a.txt"
  run run_statusline "$(make_payload "$REPO" tboundary1 50)"
  [[ "$(strip_ansi "$output")" == *"+1 ~1"* ]]

  printf 'two' >"$REPO/b.txt"
  git -C "$REPO" add b.txt
  git -C "$REPO" commit -qm second
  printf 'y' >>"$REPO/b.txt"
  # Default CACHE_TTL is 1; backdating the cache file's mtime by exactly 1
  # second hits the `>=` boundary deterministically, no sleep.
  backdate_mtime "$FAKE_HOME/.claude/cache/statusline/git-tboundary1" 1
  run run_statusline "$(make_payload "$REPO" tboundary1 50)"
  [[ "$(strip_ansi "$output")" == *"+2 ~2"* ]]
}

@test "git status cache is reused comfortably under the TTL boundary" {
  # A fresh (not backdated) cache file under a wide TTL - the only way to
  # assert the reuse side without the elapsed-time race described above.
  export STATUSLINE_CACHE_TTL=3600
  printf 'one' >"$REPO/a.txt"
  git -C "$REPO" add a.txt
  git -C "$REPO" commit -qm init
  printf 'x' >>"$REPO/a.txt"
  run run_statusline "$(make_payload "$REPO" tboundary2 50)"
  [[ "$(strip_ansi "$output")" == *"+1 ~1"* ]]

  printf 'two' >"$REPO/b.txt"
  git -C "$REPO" add b.txt
  git -C "$REPO" commit -qm second
  printf 'y' >>"$REPO/b.txt"
  backdate_mtime "$FAKE_HOME/.claude/cache/statusline/git-tboundary2" 60
  run run_statusline "$(make_payload "$REPO" tboundary2 50)"
  [[ "$(strip_ansi "$output")" == *"+1 ~1"* ]]
}

@test "a non-numeric STATUSLINE_CACHE_TTL falls back instead of blanking the line" {
  export STATUSLINE_CACHE_TTL=notanumber
  printf 'one' >"$REPO/a.txt"
  git -C "$REPO" add a.txt
  git -C "$REPO" commit -qm init
  printf 'x' >>"$REPO/a.txt"
  run run_statusline "$(make_payload "$REPO" tbadttl 50)"
  [ "$status" -eq 0 ]
  [[ "$(strip_ansi "$output")" == *"+1 ~1"* ]]
}

@test "missing jq prints a notice instead of erroring or blanking" {
  # A PATH with nothing at all also hides `cat`, which the script needs to
  # read its own stdin before it ever gets to the jq check - so build a
  # minimal stub PATH that has `cat` (via symlink to the real binary) but
  # not jq, rather than an empty PATH.
  local stub_dir="$BATS_TEST_TMPDIR/stub_no_jq"
  mkdir -p "$stub_dir"
  ln -s "$(command -v cat)" "$stub_dir/cat"
  local bash_bin
  bash_bin="$(command -v bash)"
  run env PATH="$stub_dir" HOME="$FAKE_HOME" "$bash_bin" "$SCRIPT" <<<'{}'
  [ "$status" -eq 0 ]
  [[ "$output" == *"jq not found"* ]]
}

# transcript-derived actual/declared model divergence
#
# The payload's model.display_name is always "Opus" (make_payload), so the
# session model short-name is "opus" throughout.

@test "actual model differs from the session model renders the yellow divergence arrow" {
  local transcript="$BATS_TEST_TMPDIR/t-actual.jsonl"
  write_transcript "$transcript" \
    "$(user_line "hi" "2026-01-01T00:00:01Z")" \
    "$(assistant_line "claude-sonnet-5" "2026-01-01T00:00:02Z")"
  run run_statusline "$(with_transcript "$(make_payload "$REPO" tact 50)" "$transcript")"
  [[ "$output" == *$'\033[33m'*"->"*"Sonnet"* ]]
}

@test "actual model differs from the session model renders the yellow divergence arrow (array-shaped message content)" {
  local transcript="$BATS_TEST_TMPDIR/t-actual-array.jsonl"
  write_transcript "$transcript" \
    "$(user_line_array "hi" "2026-01-01T00:00:01Z")" \
    "$(assistant_line "claude-sonnet-5" "2026-01-01T00:00:02Z")"
  run run_statusline "$(with_transcript "$(make_payload "$REPO" tactarr 50)" "$transcript")"
  [[ "$output" == *$'\033[33m'*"->"*"Sonnet"* ]]
}

@test "a sidechain (subagent) assistant entry is excluded from the actual model" {
  local transcript="$BATS_TEST_TMPDIR/t-sidechain.jsonl"
  write_transcript "$transcript" \
    "$(user_line "hi" "2026-01-01T00:00:01Z")" \
    "$(assistant_line "claude-sonnet-5" "2026-01-01T00:00:02Z" false)" \
    "$(assistant_line "claude-haiku-4-5" "2026-01-01T00:00:03Z" true)"
  run run_statusline "$(with_transcript "$(make_payload "$REPO" tside 50)" "$transcript")"
  [[ "$output" == *"Sonnet"* ]]
  [[ "$output" != *"Haiku"* ]]
}

@test "a declared override that silently falls back to the session model shows only the declared marker" {
  local transcript="$BATS_TEST_TMPDIR/t-declared-fallback.jsonl"
  write_transcript "$transcript" \
    "$(user_line "hi" "2026-01-01T00:00:01Z")" \
    "$(assistant_line "claude-opus-5" "2026-01-01T00:00:02Z")" \
    "$(declared_line "claude-sonnet-5" "2026-01-01T00:00:03Z")"
  run run_statusline "$(with_transcript "$(make_payload "$REPO" tdecl1 50)" "$transcript")"
  [[ "$output" == *$'\033[31m'"!Sonnet"* ]]
  [[ "$output" != *"->"* ]]
}

@test "a declared override that diverges from both the session and actual model shows the three-way arrow" {
  local transcript="$BATS_TEST_TMPDIR/t-declared-diverge.jsonl"
  write_transcript "$transcript" \
    "$(user_line "hi" "2026-01-01T00:00:01Z")" \
    "$(assistant_line "claude-haiku-4-5" "2026-01-01T00:00:02Z")" \
    "$(declared_line "claude-sonnet-5" "2026-01-01T00:00:03Z")"
  run run_statusline "$(with_transcript "$(make_payload "$REPO" tdecl2 50)" "$transcript")"
  [[ "$output" == *"Sonnet"*"->"*"Haiku"* ]]
}

@test "a declared attachment older than the last user prompt is ignored as stale" {
  local transcript="$BATS_TEST_TMPDIR/t-stale.jsonl"
  write_transcript "$transcript" \
    "$(declared_line "claude-sonnet-5" "2026-01-01T00:00:01Z")" \
    "$(user_line "hi" "2026-01-01T00:00:02Z")" \
    "$(assistant_line "claude-opus-5" "2026-01-01T00:00:03Z")"
  run run_statusline "$(with_transcript "$(make_payload "$REPO" tstale 50)" "$transcript")"
  [[ "$output" != *"!"* ]]
  [[ "$output" != *"->"* ]]
}

@test "a missing transcript file renders the plain row with no divergence segment" {
  run run_statusline "$(with_transcript "$(make_payload "$REPO" tmiss 50)" "$BATS_TEST_TMPDIR/does-not-exist.jsonl")"
  [ "$status" -eq 0 ]
  [[ "$(strip_ansi "$output" | head -1)" == "Opus  "* ]]
  [[ "$output" != *"!"* ]]
  [[ "$output" != *"->"* ]]
}
