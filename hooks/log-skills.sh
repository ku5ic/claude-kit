#!/usr/bin/env bash
# Appends one JSONL line per skill activation to skills.jsonl. Covers three
# paths: UserPromptExpansion (user typed /skillname directly), PostToolUse
# Skill (Claude invoked the Skill tool), and PostToolUse Read - the primary
# signal, direct SKILL.md reads, which is what guard-skills.sh actually
# checks the log for.

HOOK_NAME="log-skills.sh"
# shellcheck source=_lib.sh
source "$(dirname "$0")/_lib.sh"

read_payload

# Superset of the routing below - widen this first if a new shape is added,
# or nothing logs and guard-skills.sh blocks everything mapped.
case "$payload" in
*SKILL.md* | *Skill* | *slash_command*) ;;
*) exit 0 ;;
esac

require_jq

# Newline-delimited jq output, not IFS=$'\t' read: that merges consecutive
# tabs and would shift later fields left whenever expansion_type is empty,
# corrupting tool_name/file_path.
mapfile -t _fields < <(
  printf '%s' "$payload" | jq -r '
    (.hook_event_name // ""),
    (.expansion_type // ""),
    (.tool_name // ""),
    (.tool_input.file_path // "")'
)
event="${_fields[0]:-}"
expansion_type="${_fields[1]:-}"
tool_name="${_fields[2]:-}"
file_path="${_fields[3]:-}"

case "$event" in
UserPromptExpansion)
  [[ "$expansion_type" == "slash_command" ]] || exit 0
  ;;
PostToolUse)
  case "$tool_name" in
  Skill) ;;
  Read) [[ "$file_path" == *"/skills/"*"/SKILL.md" ]] || exit 0 ;;
  *) exit 0 ;;
  esac
  ;;
*)
  exit 0
  ;;
esac

log_dir="$HOME/.claude/logs"
mkdir -p "$log_dir"
log_file="$log_dir/skills.jsonl"

ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

printf '%s' "$payload" | jq -c \
  --arg ts "$ts" \
  '{
     ts: $ts,
     event: (.hook_event_name // null),
     session_id: (.session_id // null),
     expansion_type: (.expansion_type // null),
     command_name: (.command_name // null),
     skill_file: (.tool_input.skill // .tool_input.file_path // null),
     tool_name: (.tool_name // null)
   }' >>"$log_file"

max_lines=10000
if (($(wc -l <"$log_file") > max_lines)); then
  tmp=$(mktemp)
  trap 'rm -f "$tmp"' EXIT
  tail -n "$max_lines" "$log_file" >"$tmp"
  mv "$tmp" "$log_file"
fi

exit 0
