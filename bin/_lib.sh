#!/usr/bin/env bash
# Single source of truth for stack sentinels. Sourced by:
#   bin/detect-stack.sh
#   bin/project-root.sh
#   hooks/inject-context.sh
#
# Previously defined STACK_SENTINELS_FULL and STACK_SENTINELS_PROJECT_ROOT
# as hand-written arrays. Both are now derived from _stacks.yml via yq so
# adding a stack or sentinel here is the only edit required.
#
# Requires: yq (mikefarah, installed via Brewfile)

# Kit root: the plugin root when installed as a plugin, else the parent of
# this bin dir (~/.claude via symlinks). Tests point it at a fake tree.
KIT_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
_STACKS_YML="$KIT_ROOT/_stacks.yml"

if ! command -v yq >/dev/null 2>&1; then
  echo "_lib.sh: yq not found; stack detection disabled" >&2
  STACK_SENTINELS_FULL=()
  STACK_SENTINELS_PROJECT_ROOT=()
  STACK_DETECT_FILES=()
  STACK_PM_LOCKFILES=()
  STACK_PM_MANAGERS=()
  resolve_package_manager() { return 0; }
  return 0
fi

# Every derived list below comes from one yq pass, cached as sourceable
# `declare -p` output for as long as _stacks.yml is unchanged. Sourcing this
# file used to spawn five yq processes (~59ms), paid by guard-bash.sh on
# every Bash tool call and by project-root.sh on every bin script.
#
# STACK_SENTINELS_FULL: every sentinel any consumer cares about - used by
# inject-context.sh for cache invalidation and by detect-stack.sh as the
# canonical union.
# STACK_SENTINELS_PROJECT_ROOT: the anchor: true subset project-root.sh walks
# ancestors with. Keep minimal; every entry slows the walk for repos without
# that sentinel.
# STACK_DETECT_FILES: union of every file that influences stack detection -
# sentinels, every path referenced in extras rules (file:, in:, any_of[].file,
# any_of[].in[]), and every package_managers lockfile, so adding tsconfig.json
# or conftest.py, or switching a project from package-lock.json to
# pnpm-lock.yaml, triggers re-detection. Without the lockfiles a switched
# project kept advertising its old [pm] tag in <repo-context> while
# resolve_package_manager (which stats lockfiles live) had already moved on.
# Known limit: only $project_root/<file> is checked, not search_dirs subdirs;
# this matches the sentinel walk scope and is intentional.
# STACK_PM_LOCKFILES / STACK_PM_MANAGERS: positional pairs from
# .package_managers, walked by resolve_package_manager.
_stacks_lists_cache="$HOME/.claude/cache/stacks-lists.bash"

# Bump on every change to the queries below or to the cache's shape. The
# mtime check only sees _stacks.yml, so without this an existing cache
# outlives a rewritten derivation and keeps serving the old lists.
_stacks_lists_format=2

# Each emitted row is "<list-tag>\t<value>" so one yq call fills all five.
_build_stacks_lists() {
  local kind value
  STACK_SENTINELS_FULL=()
  STACK_SENTINELS_PROJECT_ROOT=()
  STACK_DETECT_FILES=()
  STACK_PM_LOCKFILES=()
  STACK_PM_MANAGERS=()

  while IFS=$'\t' read -r kind value; do
    [[ -z "$value" || "$value" == "null" ]] && continue
    case "$kind" in
    FULL) STACK_SENTINELS_FULL+=("$value") ;;
    ANCHOR) STACK_SENTINELS_PROJECT_ROOT+=("$value") ;;
    DETECT) STACK_DETECT_FILES+=("$value") ;;
    LOCKFILE) STACK_PM_LOCKFILES+=("$value") ;;
    MANAGER) STACK_PM_MANAGERS+=("$value") ;;
    esac
  done < <(
    yq -r '
      [
        (.stacks[].sentinels[] | ["FULL", .name]),
        (.stacks[].sentinels[] | select(.anchor == true) | ["ANCHOR", .name]),
        (.stacks[].sentinels[] | ["DETECT", .name]),
        (.stacks[].extras[]? | select(has("file")) | ["DETECT", .file]),
        (.stacks[].extras[]? | select(has("in")) | .in[] | ["DETECT", .]),
        (.stacks[].extras[]? | .any_of // [] | .[] | select(has("file")) | ["DETECT", .file]),
        (.stacks[].extras[]? | .any_of // [] | .[] | select(has("in")) | .in[] | ["DETECT", .]),
        (.package_managers[] | ["DETECT", .lockfile]),
        (.package_managers[] | ["LOCKFILE", .lockfile]),
        (.package_managers[] | ["MANAGER", .manager])
      ] | .[] | join("\t")
    ' "$_STACKS_YML" 2>/dev/null
  )

  # DETECT lists the same sentinel once per stack that declares it; the
  # previous build deduped with `sort -u`.
  local -A seen=()
  local -a uniq=()
  local f
  for f in "${STACK_DETECT_FILES[@]}"; do
    [[ -n "${seen[$f]:-}" ]] && continue
    seen[$f]=1
    uniq+=("$f")
  done
  STACK_DETECT_FILES=("${uniq[@]}")
}

_stacks_lists_cached_format=""
if [[ -s "$_stacks_lists_cache" && "$_stacks_lists_cache" -nt "$_STACKS_YML" ]]; then
  # shellcheck disable=SC1090
  source "$_stacks_lists_cache"
fi

if [[ "$_stacks_lists_cached_format" != "$_stacks_lists_format" ]]; then
  _build_stacks_lists
  _stacks_lists_cached_format="$_stacks_lists_format"
  # The temp file goes in the cache dir, not TMPDIR: mv is only atomic
  # within one filesystem, and a half-written cache is sourceable garbage.
  if ((${#STACK_SENTINELS_FULL[@]} > 0)); then
    mkdir -p "$(dirname "$_stacks_lists_cache")" 2>/dev/null || true
    _stacks_tmp="$(mktemp "$(dirname "$_stacks_lists_cache")/.stacks-lists.XXXXXX" 2>/dev/null || true)"
    if [[ -n "${_stacks_tmp:-}" ]]; then
      # -g: sourcing this file from inside a function would otherwise scope
      # every array to that function and hand the caller empty lists.
      if declare -p STACK_SENTINELS_FULL STACK_SENTINELS_PROJECT_ROOT \
        STACK_DETECT_FILES STACK_PM_LOCKFILES STACK_PM_MANAGERS \
        _stacks_lists_cached_format |
        sed -E -e 's/^declare -- /declare -g /' -e 's/^declare -([aA])/declare -g\1/' \
          >"$_stacks_tmp" 2>/dev/null; then
        mv "$_stacks_tmp" "$_stacks_lists_cache" 2>/dev/null || rm -f "$_stacks_tmp"
      else
        rm -f "$_stacks_tmp"
      fi
      unset _stacks_tmp
    fi
  fi
fi

# resolve_package_manager <dir>
# Prints the package manager name for <dir> by walking the package_managers
# table in _stacks.yml (first lockfile match wins). Checks <dir> first, then
# the git toplevel of <dir> to handle monorepos where lockfiles live at the
# root. Prints nothing when no lockfile is found; callers should apply their
# own default (e.g. npm) when empty output means "no preference".
resolve_package_manager() {
  local dir="${1:-.}"
  local toplevel
  toplevel="$(git -C "$dir" rev-parse --show-toplevel 2>/dev/null || true)"

  local i lf mgr
  for ((i = 0; i < ${#STACK_PM_LOCKFILES[@]}; i++)); do
    lf="${STACK_PM_LOCKFILES[$i]}"
    mgr="${STACK_PM_MANAGERS[$i]}"
    [[ -z "$lf" || "$lf" == "null" ]] && continue
    [[ -z "$mgr" || "$mgr" == "null" ]] && continue
    if [[ -f "$dir/$lf" || (-n "$toplevel" && -f "$toplevel/$lf") ]]; then
      printf '%s\n' "$mgr"
      return 0
    fi
  done
}

# Shared stack-cache and skill-derivation logic. Consumed by:
#   hooks/inject-context.sh (SessionStart hook, main session)
#   bin/agent-context.sh (run by the SubagentStart hook for subagents)
# Kept here so the two never carry a private copy of the same yq queries.

# stack_cache_file <project_name> <project_root>
# Prints the cache file path for a project's detect-stack.sh output. The
# project root is hashed into the filename so same-basename projects
# elsewhere on disk cannot collide.
stack_cache_file() {
  local project_name="$1" project_root="$2"
  printf '%s/%s-%s.txt\n' \
    "$HOME/.claude/cache/stack" \
    "$project_name" \
    "$(printf '%s' "$project_root" | shasum -a 256 | cut -c1-8)"
}

# refresh_stack_cache_if_stale <project_root> <cache_file>
# Regenerates the cache by running detect-stack.sh when any detection-relevant
# file (STACK_DETECT_FILES) is newer than the cache, or the cache is
# empty/missing. No output; callers read $cache_file afterward.
#
# _stacks.yml counts as detection-relevant: it defines what detect-stack.sh
# looks for, so adding a stack, sentinel or extra must re-detect every
# project. Without it a cached project kept serving the old detection until
# one of its own sentinel files happened to be touched, while the
# stacks-lists and skill-map caches had already moved on.
refresh_stack_cache_if_stale() {
  local project_root="$1" cache_file="$2"
  mkdir -p "$(dirname "$cache_file")"

  # -L on both stat forms: $_STACKS_YML is the ~/.claude symlink into the
  # dotfiles repo, and an undereferenced stat reports the link's own mtime,
  # which never changes when the file behind it is edited.
  local newest_sentinel=0 f m
  for f in "${STACK_DETECT_FILES[@]/#/$project_root/}" "$_STACKS_YML"; do
    [[ -f "$f" ]] || continue
    m="$(stat -L -c '%Y' "$f" 2>/dev/null || stat -L -f '%m' "$f" 2>/dev/null || echo 0)"
    ((m > newest_sentinel)) && newest_sentinel="$m"
  done

  local cache_mtime=0
  [[ -f "$cache_file" ]] && cache_mtime="$(stat -c '%Y' "$cache_file" 2>/dev/null || stat -f '%m' "$cache_file" 2>/dev/null || echo 0)"

  if ((cache_mtime < newest_sentinel)) || [[ ! -s "$cache_file" ]]; then
    local tmp
    tmp="$(mktemp)"
    trap 'rm -f "$tmp"' RETURN
    if bash "$KIT_ROOT/bin/detect-stack.sh" >"$tmp" 2>/dev/null; then
      mv "$tmp" "$cache_file"
    else
      rm -f "$tmp"
    fi
  fi
}

# stacks_signals_from_cache <cache_file>
# Parses a detect-stack.sh cache file into stack and stack+extra signal
# tokens, one per line, in first-seen order. Example: a cache line
# "js: yes at frontend/ (typescript, react) [pnpm]" yields "js", "js+typescript",
# "js+react".
stacks_signals_from_cache() {
  local cache="$1"
  local line stack extras extra_token
  local -a extra_tokens
  while IFS= read -r line; do
    [[ "$line" =~ ^root: ]] && continue
    [[ -z "$line" ]] && continue
    stack="${line%%:*}"
    printf '%s\n' "$stack"
    extras=$(echo "$line" | grep -oE '\([^)]+\)' | head -1 | tr -d '()') || true
    if [[ -n "$extras" ]]; then
      IFS=', ' read -ra extra_tokens <<<"$extras"
      for extra_token in "${extra_tokens[@]}"; do
        extra_token="${extra_token//[[:space:]]/}"
        [[ -n "$extra_token" ]] && printf '%s\n' "${stack}+${extra_token}"
      done
    fi
  done <"$cache"
}

# global_skills_list <yml>
# Prints _stacks.yml's global_skills, deduped, in first-seen order.
global_skills_list() {
  local yml="$1"
  local -A seen=()
  local skill
  while IFS= read -r skill; do
    [[ -z "$skill" ]] && continue
    if [[ -z "${seen[$skill]:-}" ]]; then
      seen[$skill]=1
      printf '%s\n' "$skill"
    fi
  done < <(yq '.global_skills // [] | .[]' "$yml" 2>/dev/null || true)
}

# suggested_skills_from_signals <yml>
# Reads stack/extra signals from stdin (one per line, as produced by
# stacks_signals_from_cache), prints the per-stack/extra skills mapped in
# _stacks.yml, deduped, first-seen order, excluding global_skills (those are
# required, not suggested).
suggested_skills_from_signals() {
  local yml="$1"
  local -A global_set=()
  local gsk
  while IFS= read -r gsk; do
    [[ -n "$gsk" ]] && global_set[$gsk]=1
  done < <(global_skills_list "$yml")

  local -A seen=()
  local sig stack extra sk
  while IFS= read -r sig; do
    [[ -z "$sig" ]] && continue
    if [[ "$sig" == *"+"* ]]; then
      stack="${sig%%+*}"
      extra="${sig##*+}"
      while IFS= read -r sk; do
        [[ -z "$sk" ]] && continue
        [[ -n "${global_set[$sk]:-}" ]] && continue
        if [[ -z "${seen[$sk]:-}" ]]; then
          seen[$sk]=1
          printf '%s\n' "$sk"
        fi
      done < <(yq ".stacks.${stack}.extras[] | select(.name == \"${extra}\") | .skills // [] | .[]" "$yml" 2>/dev/null || true)
    else
      while IFS= read -r sk; do
        [[ -z "$sk" ]] && continue
        [[ -n "${global_set[$sk]:-}" ]] && continue
        if [[ -z "${seen[$sk]:-}" ]]; then
          seen[$sk]=1
          printf '%s\n' "$sk"
        fi
      done < <(yq ".stacks.${sig}.skills // [] | .[]" "$yml" 2>/dev/null || true)
    fi
  done
}

# render_required_skills_block <yml>
# Prints the <required-skills> block for global_skills. Shared by
# inject-context.sh and agent-context.sh so a subagent sees the identical
# BLOCKING framing CLAUDE.md's "Required skills" section keys off of -
# agent-context.sh previously emitted a plain "skills-to-load:" list with no
# such marker, so agents had no signal to treat it as mandatory rather than
# informational.
render_required_skills_block() {
  local yml="$1"
  local -a required=()
  mapfile -t required < <(global_skills_list "$yml")
  [[ ${#required[@]} -eq 0 ]] && return 0

  local IFS=', '
  echo ""
  echo "<required-skills>"
  echo "BLOCKING REQUIREMENT: invoke the Skill tool for each of these skills NOW, before any other action: ${required[*]}"
  echo "</required-skills>"
}

# render_suggested_skills_block <yml> <cache>
# Prints the <suggested-skills> block derived from stack signals in <cache>.
# Shared by inject-context.sh and agent-context.sh; see
# render_required_skills_block for why sharing this format matters.
render_suggested_skills_block() {
  local yml="$1" cache="$2"
  [[ -s "$cache" ]] || return 0

  local -a suggested=()
  mapfile -t suggested < <(stacks_signals_from_cache "$cache" | suggested_skills_from_signals "$yml")
  [[ ${#suggested[@]} -eq 0 ]] && return 0

  echo ""
  echo "<suggested-skills>"
  local sk trigger
  for sk in "${suggested[@]}"; do
    trigger="$(yq ".skill_triggers.\"${sk}\" // \"\"" "$yml" 2>/dev/null || true)"
    if [[ -n "$trigger" && "$trigger" != "null" ]]; then
      echo "${trigger}: load ${sk} via the Skill tool"
    else
      echo "load ${sk} via the Skill tool"
    fi
  done
  if [[ "${CLAUDE_GUARD_SKILLS:-0}" == "1" ]]; then
    echo "Patterns skills are also enforced automatically: the first edit to a matching file type will be blocked until the relevant skill is loaded."
  fi
  echo "</suggested-skills>"
}
