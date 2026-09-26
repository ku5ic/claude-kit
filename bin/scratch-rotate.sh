#!/usr/bin/env bash
# Prunes ~/.claude/scratch/ artifacts older than 30 days, plus every
# project-scoped scratch/ dir registered in scratch-registry.txt (see
# scratch-dir.sh - this run has no project cwd of its own, only $HOME, so
# the registry is the only way it finds those directories).
# Also trims every ~/.claude/logs/*.jsonl to kit.yml's log_max_lines.
# Run manually or wire to launchd. Safe to run repeatedly; idempotent.
#
#   scratch-rotate.sh             # 30 days (default)
#   scratch-rotate.sh 14          # 14 days
#   scratch-rotate.sh --dry-run   # report what would go, delete nothing
#
# The project tier deletes files of any extension at any depth, so every
# registry line is validated before find touches it: absolute, under $HOME,
# a real directory named exactly `scratch`, not a symlink, not a repo. A
# single bad line would otherwise mean unrecoverable mass deletion.
# Every deleted path is appended to the rotate log so an accident stays
# diagnosable after the fact.
#
# Registry entries are absolute, so <root>/scratch rows left from the
# pre-.claude/ layout stay valid and keep getting pruned until those dirs go.

set -euo pipefail

# shellcheck source=_lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"

scratch_dir="$KIT_SCRATCH_HOME"
registry="$KIT_LOG_DIR/scratch-registry.txt"
log_file="$KIT_LOG_DIR/scratch-rotate.log"
kit_stacks_load
max_lines="$KIT_LOG_MAX_LINES"

dry_run=false
days=30
for arg in "$@"; do
  case "$arg" in
  --dry-run) dry_run=true ;;
  *) days="$arg" ;;
  esac
done

if ! [[ "$days" =~ ^[1-9][0-9]*$ ]]; then
  echo "scratch-rotate: retention window must be a positive integer, got '$days'" >&2
  exit 2
fi

delete_op=(-delete)
verb="deleted"
if $dry_run; then
  delete_op=()
  verb="would-delete"
fi

mkdir -p "$(dirname "$log_file")"

# Prints the number of matched files and records each one in the rotate log.
prune() {
  local dir="$1" age="$2"
  shift 2
  local -a filters=("$@")
  local matched

  matched="$(find "$dir" -type f "${filters[@]}" -mtime +"$age" -print "${delete_op[@]}")"
  if [[ -z "$matched" ]]; then
    echo 0
    return
  fi

  local prefix
  prefix="$(date -u +%FT%TZ) $verb "
  printf '%s%s\n' "$prefix" "${matched//$'\n'/$'\n'$prefix}" >>"$log_file"
  printf '%s\n' "$matched" | wc -l | tr -d ' '
}

# A registry line is an arbitrary string on disk; treat it as untrusted.
valid_scratch_dir() {
  local dir="$1"
  [[ "$dir" == /* ]] || return 1
  [[ "$dir" != *..* ]] || return 1
  [[ "$dir" == "$HOME"/* ]] || return 1
  [[ "${dir##*/}" == scratch ]] || return 1
  [[ -d "$dir" && ! -L "$dir" ]] || return 1
  [[ ! -e "$dir/.git" ]] || return 1
}

if [[ -d "$scratch_dir" ]]; then
  # Prune .md artifacts (configurable retention) and stale .injected-*
  # session markers (fixed 1-day retention - they're only needed to dedupe
  # injection within a session's lifetime, far shorter than artifact
  # retention).
  removed="$(prune "$scratch_dir" "$days" -name '*.md')"
  markers_removed="$(prune "$scratch_dir" 1 -maxdepth 1 -name '.injected-*')"

  echo "scratch-rotate: pruned $removed artifact(s) older than ${days}d from $scratch_dir"
  echo "scratch-rotate: pruned $markers_removed session marker(s) older than 1d from $scratch_dir"
fi

skills_loaded_cache="$KIT_CACHE_DIR/skills-loaded"
if [[ -d "$skills_loaded_cache" ]]; then
  loaded_removed="$(prune "$skills_loaded_cache" 1)"
  echo "scratch-rotate: pruned $loaded_removed skill-loaded marker(s) older than 1d from $skills_loaded_cache"
fi

if [[ -f "$registry" ]]; then
  # Project scratch/ dirs hold test artifacts and POC files of any
  # extension, not just reports, so prune by age alone - no name filter.
  tmp_registry="$(mktemp)"
  while IFS= read -r proj_dir; do
    [[ -z "$proj_dir" ]] && continue
    if [[ ! -d "$proj_dir" ]]; then
      echo "scratch-rotate: dropping stale registry entry $proj_dir (directory no longer exists)"
      continue
    fi
    if ! valid_scratch_dir "$proj_dir"; then
      echo "scratch-rotate: REFUSING registry entry $proj_dir (not a plain scratch/ dir under \$HOME)" >&2
      echo "$proj_dir" >>"$tmp_registry"
      continue
    fi
    proj_removed="$(prune "$proj_dir" "$days")"
    echo "scratch-rotate: pruned $proj_removed artifact(s) older than ${days}d from $proj_dir"
    echo "$proj_dir" >>"$tmp_registry"
  done <"$registry"
  $dry_run || mv "$tmp_registry" "$registry"
fi

# Every JSONL log the kit writes (skills.jsonl, guards.jsonl) is capped here;
# the hooks that append to them never trim.
for jsonl in "$KIT_LOG_DIR"/*.jsonl; do
  [[ -f "$jsonl" ]] || continue
  name="${jsonl##*/}"
  total="$(wc -l <"$jsonl" | tr -d ' ')"
  if ((total > max_lines)); then
    if $dry_run; then
      echo "scratch-rotate: would trim $name from $total to $max_lines lines"
    else
      tmp="$(mktemp)"
      tail -n "$max_lines" "$jsonl" >"$tmp"
      mv "$tmp" "$jsonl"
      echo "scratch-rotate: trimmed $name from $total to $max_lines lines"
    fi
  else
    echo "scratch-rotate: $name has $total lines, no trim needed"
  fi
done
