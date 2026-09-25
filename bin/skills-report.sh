#!/usr/bin/env bash
# Reports skill activation telemetry from ~/.claude/logs/skills.jsonl.
# Read-only: never writes, trims, or rotates the log (see scratch-rotate.sh
# for trimming). Malformed JSONL lines are counted and reported, never fatal.
#
# Reports, over a window defaulting to 30 days (override as the first argument):
#   1. Activation count per skill, descending
#   2. Counts split by path: slash command, Skill tool invocation, Read
#      fallback, plus a separate "surfaced only" section for the
#      required-skill/suggested-skill synthetic markers inject-context.sh
#      writes (those mean "shown to the model", not "confirmed loaded", and
#      are never folded into the three real-activation counts above)
#   3. Skills referenced anywhere in _stacks.yml (global_skills, per-stack
#      skills, per-extra skills, skill_file_map) with zero activations in
#      the window
#   4. Skills with activations in the window that appear nowhere in
#      _stacks.yml (expected for audit, write, meta-*, deps, and
#      explore-patterns procedure skills, which _stacks.yml never maps -- not a defect)
#   5. Sessions where a stack-suggested skill was surfaced (a
#      "suggested-skill" log entry) but never activated in that same
#      session -- only measurable for entries after inject-context.sh started
#      emitting suggested-skill markers; earlier sessions have no marker and
#      are silently excluded here, not counted as followed
#
# log-skills.sh logs one PostToolUse+Skill entry per Skill-tool invocation;
# that entry is the Skill-tool activation count.
#
# Usage:
#   skills-report.sh        # last 30 days
#   skills-report.sh 7      # last 7 days

set -euo pipefail

days="${1:-30}"
log_file="$HOME/.claude/logs/skills.jsonl"
stacks_yml="${CLAUDE_PLUGIN_ROOT:-$(dirname "${BASH_SOURCE[0]}")/..}/_stacks.yml"

if ! command -v jq >/dev/null 2>&1; then
  echo "skills-report: jq not found, cannot generate report"
  exit 0
fi

if [[ ! -f "$log_file" ]]; then
  echo "skills-report: no log at $log_file, nothing to report"
  exit 0
fi

if [[ ! -s "$log_file" ]]; then
  echo "skills-report: $log_file is empty, nothing to report"
  exit 0
fi

# Single jq pass over the raw file: blank lines are dropped (not data, not
# malformed), everything else is parsed or replaced with a sentinel so one
# bad line never aborts the whole read.
classified="$(jq -R -c 'select(length > 0) | try fromjson catch "__MALFORMED__"' "$log_file")"
malformed="$(grep -c '^"__MALFORMED__"$' <<<"$classified" || true)"
valid_stream="$(grep -v '^"__MALFORMED__"$' <<<"$classified" || true)"

if [[ -z "$valid_stream" ]]; then
  echo "skills-report: no valid JSONL lines in $log_file ($malformed malformed)"
  exit 0
fi

cutoff="$(date -u -v-"${days}"d +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "-${days} days" +%Y-%m-%dT%H:%M:%SZ)"

entries="$(jq -s --arg cutoff "$cutoff" '[.[] | select(.ts >= $cutoff)]' <<<"$valid_stream")"
entry_count="$(jq 'length' <<<"$entries")"

echo "skills-report: window=${days}d cutoff=${cutoff} entries=${entry_count} malformed=${malformed}"
echo

if [[ "$entry_count" -eq 0 ]]; then
  echo "no entries in the last ${days} day(s)"
  exit 0
fi

augmented="$(jq '
  def skill_name:
    if .category == "slash_command" then
      (.command_name // "" | ltrimstr("/") | sub(":.*$"; ""))
    elif .category == "read_fallback" then
      (try (.skill_file | capture("/skills/(?<n>[^/]+)/SKILL\\.md$").n) catch .skill_file)
    else
      .skill_file
    end;
  [.[]
   | . + {category: (
       if .event == "UserPromptExpansion" and .expansion_type == "slash_command" then "slash_command"
       elif .event == "PostToolUse" and .tool_name == "Skill" then "skill_tool"
       elif .event == "PostToolUse" and .tool_name == "Read" then "read_fallback"
       elif .event == "required-skill" then "surfaced_required"
       elif .event == "suggested-skill" then "surfaced_suggested"
       else "ignored"
       end)}
   | . + {skill: skill_name}
   | select(.category != "ignored")]
' <<<"$entries")"

echo "== 1+2: activation counts per skill, by path (real activations only) =="
counts="$(jq -r '
  [.[] | select(.category=="slash_command" or .category=="skill_tool" or .category=="read_fallback")]
  | group_by(.skill)
  | map({skill: .[0].skill, count: length,
         slash_command: ([.[] | select(.category=="slash_command")] | length),
         skill_tool: ([.[] | select(.category=="skill_tool")] | length),
         read_fallback: ([.[] | select(.category=="read_fallback")] | length)})
  | sort_by(-.count)
  | .[] | "\(.count)  \(.skill)  (slash=\(.slash_command) skill_tool=\(.skill_tool) read=\(.read_fallback))"
' <<<"$augmented")"
if [[ -z "$counts" ]]; then
  echo "(no real activations in the window)"
else
  printf '%s\n' "$counts"
fi

echo
echo "== 2b: surfaced only, not confirmed loaded (required-skill / suggested-skill markers) =="
surfaced="$(jq -r '
  [.[] | select(.category=="surfaced_required" or .category=="surfaced_suggested")]
  | group_by(.skill)
  | map({skill: .[0].skill,
         required: ([.[] | select(.category=="surfaced_required")] | length),
         suggested: ([.[] | select(.category=="surfaced_suggested")] | length)})
  | sort_by(-(.required + .suggested))
  | .[] | "\(.skill)  required=\(.required) suggested=\(.suggested)"
' <<<"$augmented")"
if [[ -z "$surfaced" ]]; then
  echo "(none in the window)"
else
  printf '%s\n' "$surfaced"
fi

if command -v yq >/dev/null 2>&1 && [[ -f "$stacks_yml" ]]; then
  mapfile -t referenced_skills < <(
    {
      yq '.global_skills // [] | .[]' "$stacks_yml"
      yq '.stacks | to_entries[] | .value.skills // [] | .[]' "$stacks_yml"
      yq '.stacks | to_entries[] | .value.extras // [] | .[].skills // [] | .[]' "$stacks_yml"
      yq '.skill_file_map // [] | .[].skills // [] | .[]' "$stacks_yml"
    } 2>/dev/null | sort -u
  )
  referenced_json="$(printf '%s\n' "${referenced_skills[@]}" | jq -R -s 'split("\n") | map(select(length > 0)) | unique')"

  echo
  echo "== 3: _stacks.yml-referenced skills with zero activations in the window =="
  zero_activation="$(jq -r --argjson referenced "$referenced_json" '
    ([.[] | select(.category=="slash_command" or .category=="skill_tool" or .category=="read_fallback") | .skill] | unique) as $active
    | ($referenced - $active) | sort | .[]
  ' <<<"$augmented")"
  if [[ -z "$zero_activation" ]]; then
    echo "(none - every referenced skill was activated at least once in the window)"
  else
    printf '%s\n' "$zero_activation"
  fi

  echo
  echo "== 4: activations for skills not referenced anywhere in _stacks.yml =="
  echo "(expected for audit, write, meta-*, deps, and explore-patterns procedure skills -- _stacks.yml only maps pattern/reference skills to stacks, not this group)"
  unreferenced="$(jq -r --argjson referenced "$referenced_json" '
    ([.[] | select(.category=="slash_command" or .category=="skill_tool" or .category=="read_fallback") | .skill] | unique) as $active
    | ($active - $referenced) | sort | .[]
  ' <<<"$augmented")"
  if [[ -z "$unreferenced" ]]; then
    echo "(none)"
  else
    printf '%s\n' "$unreferenced"
  fi
else
  echo
  echo "== 3+4: skipped (yq or _stacks.yml not available) =="
fi

echo
echo "== 5: sessions with a suggested skill surfaced but never activated in that session =="
unfollowed="$(jq -r '
  ([.[] | select(.category=="slash_command" or .category=="skill_tool" or .category=="read_fallback") | "\(.session_id)::\(.skill)"]) as $activated
  | [.[] | select(.category=="surfaced_suggested")]
  | group_by(.session_id)
  | map({
      session_id: .[0].session_id,
      unfollowed: ([.[] | . as $e | select(($activated | index("\($e.session_id)::\($e.skill)")) == null) | $e.skill] | unique)
    })
  | map(select(.unfollowed | length > 0))
  | .[] | "\(.session_id): \(.unfollowed | join(", "))"
' <<<"$augmented")"
if [[ -z "$unfollowed" ]]; then
  echo "(none - either no suggested-skill markers in the window, or every one was followed)"
else
  printf '%s\n' "$unfollowed"
fi
echo "(sessions before suggested-skill logging landed have no marker and are excluded above, not counted as followed)"
