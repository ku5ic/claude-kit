#!/usr/bin/env bash
# Appends one JSONL line per skill activation to skills.jsonl. Covers three
# paths: UserPromptExpansion (user typed /skillname directly), PostToolUse
# Skill (Claude invoked the Skill tool), and PostToolUse Read - the primary
# signal, direct SKILL.md reads, which is what guard-skills.sh actually
# checks the log for.

HOOK_NAME="log-skills.sh"
# shellcheck source=../bin/_lib.sh
source "$(dirname "$0")/../bin/_lib.sh"
kit_hook_init

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
    (.tool_input.file_path // ""),
    (.command_name // ""),
    (.tool_input.skill // .tool_input.file_path // "")'
)
event="${_fields[0]:-}"
expansion_type="${_fields[1]:-}"
tool_name="${_fields[2]:-}"
file_path="${_fields[3]:-}"
command_name="${_fields[4]:-}"
skill_file="${_fields[5]:-}"

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

# scratch-rotate.sh trims the log to log_max_lines.
log_event skills "$event" \
  expansion_type "$expansion_type" \
  command_name "$command_name" \
  skill_file "$skill_file" \
  tool_name "$tool_name"

exit 0
