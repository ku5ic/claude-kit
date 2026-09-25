#!/usr/bin/env bash
# statusLine command (see claude/settings.json). Reads the payload Claude Code
# pipes to stdin and prints a two-row status: model/agent/dir/git on row 1,
# context/cost/effort/rate-limit on row 2. Never blocks a render: a missing
# jq, a missing field, or any other unexpected error falls through to a
# best-effort or blank line rather than raising.

set -euo pipefail
trap 'exit 0' ERR

# Seconds the git-status cache is reused. 1 matches statusLine.refreshInterval
# in settings.json, so the git segment can't be more than one render stale.
# Overridable so tests can pin a wide window and assert cache reuse without
# racing the clock; a non-numeric value would make the `(( ))` age comparison
# below abort the whole render, so it falls back rather than trusting it.
CACHE_TTL="${STATUSLINE_CACHE_TTL:-1}"
[[ "$CACHE_TTL" =~ ^[0-9]+$ ]] || CACHE_TTL=1
readonly CACHE_TTL
readonly BAR_WIDTH=10 # blocks in the context-usage bar
readonly YELLOW_AT=70 # bar turns yellow at >=70% context used
readonly RED_AT=90    # bar turns red at >=90% context used

payload="$(cat)"

if ! command -v jq >/dev/null 2>&1; then
  echo "statusline: jq not found"
  exit 0
fi

# One value per line, read via a while-read loop rather than `IFS=$'\t' read`
# (tab is IFS whitespace, so `read` collapses consecutive empty tab fields and
# silently shifts every later value one slot left) or `mapfile` (bash 4+ only;
# `env bash` can resolve to the stock macOS bash 3.2 if Homebrew's bash isn't
# first on the invoking process's PATH).
fields=()
while IFS= read -r line; do
  fields+=("$line")
done < <(
  jq -r '
    (.model.display_name // "unknown"),
    (.workspace.current_dir // ""),
    (.session_id // ""),
    (.context_window.used_percentage // 0),
    (.cost.total_cost_usd // 0),
    (.cost.total_duration_ms // 0),
    (.effort.level // ""),
    (.rate_limits.five_hour.used_percentage // ""),
    (.agent.name // .agent_type // ""),
    (.transcript_path // "")
  ' <<<"$payload"
)
# Empty or invalid-JSON stdin makes jq exit non-zero with no stdout, leaving
# fields short; pad to the 10 values above so indexing below can't hit
# `set -u`'s unbound-variable error (which bypasses the ERR trap).
while ((${#fields[@]} < 10)); do fields+=(""); done
model_name="${fields[0]}"
cwd="${fields[1]}"
session_id="${fields[2]}"
ctx_pct="${fields[3]}"
cost_usd="${fields[4]}"
duration_ms="${fields[5]}"
effort_level="${fields[6]}"
five_h="${fields[7]}"
# Both keys carry the main thread's agent type and are absent for a plain
# interactive session; reading both keeps this working if they ever diverge.
agent_name="${fields[8]}"
transcript_path="${fields[9]}"

# model_display_from_id <raw-model-id>
# Converts a raw model id ("claude-opus-5", "claude-haiku-4-5-20251001") into
# the same display form the payload's own model.display_name uses ("Opus 5",
# "Haiku 4.5"): drop the claude- prefix and any trailing long numeric release
# date, dot-join the remaining version segments, and capitalize the family
# name. No `${var^}` (bash 4+ only): this file avoids bash-4-only syntax since
# `env bash` can resolve to the stock macOS bash 3.2 (see the field-read note
# above).
model_display_from_id() {
  local id="$1" family version
  id="${id#claude-}"
  id="$(printf '%s' "$id" | sed -E 's/-[0-9]{8,}$//')"
  family="${id%%-*}"
  if [[ "$id" == *-* ]]; then
    version="${id#*-}"
    version="${version//-/.}"
  else
    version=""
  fi
  family="$(printf '%s%s' "$(printf '%s' "${family:0:1}" | tr '[:lower:]' '[:upper:]')" "${family:1}")"
  if [[ -n "$version" ]]; then
    printf '%s %s' "$family" "$version"
  else
    printf '%s' "$family"
  fi
}

# Actual running model, read from the transcript rather than the payload: the
# payload's model.* is the session model and never reflects a skill's per-turn
# model override. The last main-thread assistant entry carries the resolved
# model id of whatever actually served that message. Sidechain (subagent)
# entries share the transcript and must be excluded.
yellow_marker=$'\033[33m'
red_marker=$'\033[31m'
reset_marker=$'\033[0m'
model_color=$'\033[38;5;111m'
dir_color=$'\033[38;5;216m'
branch_color=$'\033[38;5;141m'
additions_color=$'\033[38;5;150m'
deletions_color=$'\033[38;5;209m'
actual_model_short=""
actual_model_display=""
session_model_short=""
declared_model_short=""
declared_model_display=""
if [[ -n "$transcript_path" && -r "$transcript_path" ]]; then
  # One tail + one jq across all three transcript-derived values instead of
  # three separate tail|jq pipelines (six processes) reading the same file
  # three times. last_user_ts is computed inside the same jq program and fed
  # straight into the declared_id filter's $since bound, rather than being
  # substituted back in as a second invocation's argument.
  t_fields=()
  while IFS= read -r line; do
    t_fields+=("$line")
  done < <(
    tail -n 60 "$transcript_path" 2>/dev/null | jq -rs '
      ([.[] | select(.type == "user" and .isMeta != true)
            | select((.message.content | if type == "string" then . else ([.[]? | select(.type == "text") | .text] | join("\n")) end | length) > 0)
            | .timestamp] | last // "") as $since |
      ([.[] | select(.type == "assistant" and .isSidechain != true) | .message.model // empty] | last // ""),
      $since,
      (if $since == "" then "" else
        ([.[] | select(.type == "attachment" and .attachment.type == "command_permissions" and .timestamp >= $since) | .attachment.model // empty] | last // "")
      end)
    ' 2>/dev/null
  )
  while ((${#t_fields[@]} < 3)); do t_fields+=(""); done
  actual_id="${t_fields[0]}"
  declared_id="${t_fields[2]}"

  if [[ -n "$actual_id" ]]; then
    actual_model_short="$(printf '%s' "$actual_id" | sed -E 's/^claude-//; s/-[0-9].*$//')"
    actual_model_display="$(model_display_from_id "$actual_id")"
    # model_name is the display name (e.g. "Sonnet 5", "Haiku 4.5") - strip the
    # trailing version number the same way actual_model_short strips it from
    # the model id, or "sonnet 5" would never equal "sonnet" and every render
    # would falsely show a divergence, same model or not.
    session_model_short="$(printf '%s' "$model_name" | tr '[:upper:]' '[:lower:]' | sed -E 's/[[:space:]]+[0-9].*$//')"

    # A skill's frontmatter model override (e.g. `model: opus`) is recorded as
    # a command_permissions attachment at invocation time, before the model
    # actually resolves - it can silently not take (unavailable model,
    # unsupported effort), with the completion running on the session model
    # anyway. Only trust the attachment as belonging to the CURRENT turn if it
    # is newer than the current turn's own user prompt; otherwise a stale
    # declaration from several turns back would get compared against an
    # unrelated later response.
    if [[ -n "$declared_id" && "$declared_id" != "$actual_id" ]]; then
      declared_model_short="$(printf '%s' "$declared_id" | sed -E 's/^claude-//; s/-[0-9].*$//')"
      declared_model_display="$(model_display_from_id "$declared_id")"
    fi
  fi
fi

dir_name="$(basename "${cwd:-.}")"

safe_session_id="${session_id//[^a-zA-Z0-9_-]/}"
cache_dir="$HOME/.claude/cache/statusline"
git_cache_file="$cache_dir/git-${safe_session_id:-nosession}"

git_segment=""
if [[ -n "$cwd" ]] && git -C "$cwd" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  mkdir -p "$cache_dir"
  now="$(date +%s)"
  cache_mtime=0
  [[ -f "$git_cache_file" ]] && cache_mtime="$(stat -c '%Y' "$git_cache_file" 2>/dev/null || stat -f '%m' "$git_cache_file" 2>/dev/null || echo 0)"
  if ((now - cache_mtime >= CACHE_TTL)) || [[ ! -s "$git_cache_file" ]]; then
    branch="$(git -C "$cwd" branch --show-current 2>/dev/null)"
    # Combines staged + unstaged numstat, matching what `git diff --shortstat`
    # reports as "N insertions, M deletions". Not `git diff HEAD` - that errors
    # on an unborn branch (no commits yet) and would hide staged content.
    read -r additions deletions <<<"$({
      git -C "$cwd" diff --numstat 2>/dev/null
      git -C "$cwd" diff --cached --numstat 2>/dev/null
    } | awk '{a+=$1; d+=$2} END {print a+0, d+0}')"
    tmp="$(mktemp)"
    # Tab-separated, not the display-ready "branch +N ~M" string: cache holds
    # data, coloring happens at render time so each part can carry its own color.
    printf '%s\t%s\t%s\n' "${branch:-detached}" "$additions" "$deletions" >"$tmp"
    mv "$tmp" "$git_cache_file"
  fi
  git_segment="$(cat "$git_cache_file" 2>/dev/null)"
fi

# Session-mode badge. ponytail persists its state to a flag file in ~/.claude
# rather than exposing it any other way, so this reads that file directly
# instead of shelling out to the plugin's own (versioned-path, so fragile to
# depend on) hook script.
mode_segment=""
ponytail_flag="$HOME/.claude/.ponytail-active"
if [[ -f "$ponytail_flag" ]]; then
  ponytail_mode="$(head -n1 "$ponytail_flag" | tr -d '[:space:]')"
  # Matches the ponytail plugin's own statusline snippet: amber for the
  # YAGNI-extremist "ultra" level, cyan for everything else.
  ponytail_color=$'\033[38;5;108m'
  [[ "$ponytail_mode" == "ultra" ]] && ponytail_color=$'\033[38;5;173m'
  [[ -z "$ponytail_mode" ]] && ponytail_mode="full"
  mode_segment="$mode_segment ${ponytail_color}[PONYTAIL:$(printf '%s' "$ponytail_mode" | tr '[:lower:]' '[:upper:]')]${reset_marker}"
fi

row1="${model_color}${model_name}${reset_marker}"
if [[ -n "$declared_model_short" ]]; then
  # actual_model_short usually equals session_model_short here (that is the
  # silent-fallback case: the override fell back to the session model, which
  # is already shown as the leading segment of row1) - only show the actual
  # side too when it is itself a third, different value worth naming.
  if [[ "$actual_model_short" == "$session_model_short" ]]; then
    row1="$row1  ${red_marker}!${declared_model_display}${reset_marker}"
  else
    row1="$row1  ${red_marker}!${declared_model_display}->  ${actual_model_display}${reset_marker}"
  fi
elif [[ -n "$actual_model_short" && "$actual_model_short" != "$session_model_short" ]]; then
  row1="$row1  ${yellow_marker}->  ${actual_model_display}${reset_marker}"
fi
[[ -n "$agent_name" ]] && row1="$row1 ($agent_name)"
row1="$row1  ${dir_color}${dir_name}${reset_marker}"
if [[ -n "$git_segment" ]]; then
  IFS=$'\t' read -r git_branch git_additions git_deletions <<<"$git_segment"
  row1="$row1  ${branch_color}${git_branch}${reset_marker} ${additions_color}+${git_additions}${reset_marker} ${deletions_color}~${git_deletions}${reset_marker}"
fi
row1="$row1$mode_segment"

ctx_int="${ctx_pct%%.*}"
[[ -z "$ctx_int" ]] && ctx_int=0
((ctx_int < 0)) && ctx_int=0
((ctx_int > 100)) && ctx_int=100
filled=$((ctx_int / (100 / BAR_WIDTH)))

if ((ctx_int >= RED_AT)); then
  color=$'\033[31m'
elif ((ctx_int >= YELLOW_AT)); then
  color=$'\033[33m'
else
  color=$'\033[32m'
fi

bar=""
for ((i = 0; i < BAR_WIDTH; i++)); do
  if ((i < filled)); then
    bar+="█"
  else
    bar+="░"
  fi
done

cost_fmt="$(printf '%.2f' "$cost_usd" 2>/dev/null || echo "$cost_usd")"

duration_s=$((duration_ms / 1000))
if ((duration_s >= 3600)); then
  duration_fmt="$((duration_s / 3600))h $((duration_s % 3600 / 60))m"
elif ((duration_s >= 60)); then
  duration_fmt="$((duration_s / 60))m"
else
  duration_fmt="${duration_s}s"
fi
# Subtle, muted gray rather than a saturated hue: duration is a secondary
# stat next to cost, not something that should compete for attention.
duration_color=$'\033[38;5;245m'

row2="${color}${bar}${reset_marker} ${ctx_int}%  \$${cost_fmt}   ${duration_color}${duration_fmt}${reset_marker}"
tail=""
[[ -n "$effort_level" ]] && tail="$tail  effort:${effort_level}"
[[ -n "$five_h" ]] && tail="$tail  5h:${five_h%%.*}%"
[[ -n "$tail" ]] && row2="$row2 $tail"

printf '%s\n%s\n' "$row1" "$row2"
