#!/usr/bin/env bash
# Shared library for every hook and bin script in the kit. Sourcing it only
# defines things; nothing runs until a caller asks:
#   - hooks call kit_hook_init for strict mode plus a fail-open ERR trap.
#     Bin scripts don't, so they never inherit the fail-open trap.
#   - the stack lists derived from kit.yml load on first use, through
#     kit_stacks_load, so hooks that never read them never pay for yq.
#
# Requires: bash 4.4+, jq, and yq (mikefarah's, not the Python one)

# Idempotency guard: guard-dispatch.sh sources this, then sources
# guard-edit.sh and guard-skills.sh, which each source it again for
# standalone use.
[[ -n "${_KIT_LIB_SOURCED:-}" ]] && return 0
_KIT_LIB_SOURCED=1

# Prerequisites. Everything up to the bash version gate below must parse and
# run under macOS /bin/bash 3.2: [ ] tests, no arrays, no bash 4 expansions.
# Every guard fails open (exit 0) without these, so callers report them:
# inject-context.sh at session start, doctor.sh on demand.
_kit_lib_dir="$(cd "${BASH_SOURCE[0]%/*}" 2>/dev/null && pwd)"

# check_prereqs: the tools. Returns 1 and sets KIT_PREREQ_MISSING, a
# "; "-separated list naming each missing piece and its install command.
check_prereqs() {
  KIT_PREREQ_MISSING=""
  if [ "${BASH_VERSINFO[0]}" -lt 4 ] || { [ "${BASH_VERSINFO[0]}" -eq 4 ] && [ "${BASH_VERSINFO[1]}" -lt 4 ]; }; then
    KIT_PREREQ_MISSING="${KIT_PREREQ_MISSING}; bash 4.4+ (found ${BASH_VERSION}; brew install bash, and put it first on PATH)"
  fi
  if ! command -v jq >/dev/null 2>&1; then
    KIT_PREREQ_MISSING="${KIT_PREREQ_MISSING}; jq (brew install jq)"
  fi
  if ! yq --version 2>/dev/null | grep -q mikefarah; then
    KIT_PREREQ_MISSING="${KIT_PREREQ_MISSING}; mikefarah yq (brew install yq)"
  fi
  KIT_PREREQ_MISSING="${KIT_PREREQ_MISSING#; }"
  [ -z "$KIT_PREREQ_MISSING" ]
}

# True when dir $1 is the kit's rules dir $2, or another copy of it holding
# every rule file $2 has. A plugin's hooks run from the versioned plugin
# cache while install-rules.sh links the marketplace clone, so the two paths
# never match.
_kit_rules_dir_matches() {
  local rule
  [ "$(cd -P "$1" 2>/dev/null && pwd)" = "$2" ] && return 0
  [ -n "$2" ] || return 1
  for rule in "$2"/*.md; do
    [ -f "$rule" ] || return 1
    [ -f "$1/${rule##*/}" ] || return 1
  done
  return 0
}

# check_install: the layout a session needs, the kit rules linked under
# ~/.claude/rules and a readable kit.yml. Same contract as check_prereqs.
check_install() {
  local kit_rules entry linked=0
  KIT_PREREQ_MISSING=""
  kit_rules="$(cd -P "$_kit_lib_dir/../rules" 2>/dev/null && pwd)"
  for entry in "${CLAUDE_CONFIG_DIR:-$HOME/.claude}"/rules/*; do
    if [ -d "$entry" ] && _kit_rules_dir_matches "$entry" "$kit_rules"; then
      linked=1
    fi
  done
  if [ "$linked" -eq 0 ]; then
    KIT_PREREQ_MISSING="${KIT_PREREQ_MISSING}; the kit rules linked under ~/.claude/rules (run bootstrap.sh)"
  fi
  if [ ! -r "${CLAUDE_PLUGIN_ROOT:-$_kit_lib_dir/..}/kit.yml" ]; then
    KIT_PREREQ_MISSING="${KIT_PREREQ_MISSING}; a readable kit.yml at the kit root (run bootstrap.sh)"
  fi
  KIT_PREREQ_MISSING="${KIT_PREREQ_MISSING#; }"
  [ -z "$KIT_PREREQ_MISSING" ]
}

# The bash version gate. On anything older than 4.4, nothing below this runs:
# before 4.4, set -u treats "${empty[@]}" as unbound, so a hook would error
# (exit 1, not a block) on its first empty list. A bin script stops with the
# reason. A hook (it sets HOOK_NAME before sourcing) gets a kit_hook_init
# that exits 0, so it fails open quietly and inject-context.sh's warning
# names the cause.
if [ "${BASH_VERSINFO[0]}" -lt 4 ] || { [ "${BASH_VERSINFO[0]}" -eq 4 ] && [ "${BASH_VERSINFO[1]}" -lt 4 ]; }; then
  if [ -z "${HOOK_NAME:-}" ]; then
    echo "${0##*/}: needs bash 4.4+ (found $BASH_VERSION); brew install bash, and put it first on PATH" >&2
    exit 1
  fi
  kit_hook_init() { exit 0; }
  return 0
fi

# This lib's own bin dir, for calling sibling scripts. Not $KIT_ROOT/bin:
# tests point CLAUDE_PLUGIN_ROOT at a fake tree with no scripts in it.
# Builtins only (no dirname): the statusline sources this under whatever
# PATH it gets, and every hook pays for each subprocess.
_KIT_BIN_DIR="$(cd "${BASH_SOURCE[0]%/*}" && pwd)"
# Kit root: the plugin root when installed as a plugin, else the parent of
# this bin dir (~/.claude via symlinks). Tests point it at a fake tree.
KIT_ROOT="${CLAUDE_PLUGIN_ROOT:-${_KIT_BIN_DIR%/*}}"

# Claude Code's config dir, relocatable with CLAUDE_CONFIG_DIR. Every path
# the kit writes under it derives from here.
KIT_HOME="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
KIT_LOG_DIR="$KIT_HOME/logs"
KIT_CACHE_DIR="$KIT_HOME/cache"
KIT_SCRATCH_HOME="$KIT_HOME/scratch"
KIT_PLANS_HOME="$KIT_HOME/plans"

# kit.yml plus the user's overlay. Maps merge and arrays append, so an overlay
# can add stacks and protections but never remove one. KIT_YML is what every
# consumer reads: the merged copy when an overlay exists, else kit.yml. The
# merged copy is regenerated whenever either input is newer, so caches keyed
# on KIT_YML's mtime go stale with either file.
KIT_YML_BASE="$KIT_ROOT/kit.yml"
KIT_OVERLAY="$KIT_HOME/claude-kit.local.yml"
_kit_resolve_yml() {
  KIT_YML="$KIT_YML_BASE"
  [[ -f "$KIT_OVERLAY" ]] || return 0
  local merged="$KIT_CACHE_DIR/kit.merged.yml"
  if [[ -s "$merged" && "$merged" -nt "$KIT_YML_BASE" && "$merged" -nt "$KIT_OVERLAY" ]]; then
    KIT_YML="$merged"
    return 0
  fi
  command -v yq >/dev/null 2>&1 || return 0
  mkdir -p "$KIT_CACHE_DIR" 2>/dev/null || return 0
  local tmp
  tmp="$(mktemp "$KIT_CACHE_DIR/.kit.merged.XXXXXX" 2>/dev/null)" || return 0
  # shellcheck disable=SC2016  # $i is a yq variable, not a shell one
  if yq eval-all '. as $i ireduce ({}; . *+ $i)' "$KIT_YML_BASE" "$KIT_OVERLAY" >"$tmp" 2>/dev/null &&
    [[ -s "$tmp" ]] && mv "$tmp" "$merged" 2>/dev/null; then
    KIT_YML="$merged"
  else
    rm -f "$tmp"
    echo "_lib.sh: could not merge $KIT_OVERLAY; using kit.yml alone" >&2
  fi
}
_kit_resolve_yml
# Caches derived from KIT_YML carry this tag in their path. Deleting the
# overlay switches KIT_YML back to the older kit.yml, and an mtime check alone
# would keep serving the overlay's lists, disabled rules included.
KIT_YML_TAG="merged"
[[ "$KIT_YML" == "$KIT_YML_BASE" ]] && KIT_YML_TAG="base"

# Physical path of $1: follows a symlink at the file itself, then resolves
# its directory. Works for paths that don't exist yet.
kit_physical_path() {
  local path="$1" target dir
  while [[ -L "$path" ]]; do
    target="$(readlink "$path")"
    [[ "$target" == /* ]] || target="${path%/*}/$target"
    path="$target"
  done
  if dir="$(cd -P "${path%/*}" 2>/dev/null && pwd)"; then
    printf '%s/%s\n' "$dir" "${path##*/}"
  else
    printf '%s\n' "$path"
  fi
}

# True when $1 is the overlay, reached through any path or symlink. Writes
# to it get a prompt: it can switch the kit's own protections off.
kit_is_overlay_path() {
  [[ "$(kit_physical_path "$1")" == "$(kit_physical_path "$KIT_OVERLAY")" ]]
}

# Prints a PreToolUse permission decision (allow or ask) with its reason.
emit_decision() {
  jq -cn --arg decision "$1" --arg reason "$2" '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: $decision,
      permissionDecisionReason: $reason
    }
  }'
}

# kit_dir scratch|plans [--no-create]
# Prints <project-root>/.claude/<kind> inside a recognized project (a git
# worktree or a stack sentinel, per project-root.sh --check), else the
# $KIT_HOME fallback. Without --no-create it creates the directory and, for
# a project scratch dir, registers it in scratch-registry.txt so
# scratch-rotate.sh's scheduled run, which has no project cwd, can prune it.
kit_dir() {
  local kind="$1" create=1 dir
  [[ "${2:-}" == --no-create ]] && create=0
  case "$kind" in
  scratch | plans) ;;
  *)
    echo "kit_dir: unknown kind: $kind" >&2
    return 2
    ;;
  esac

  if "$_KIT_BIN_DIR/project-root.sh" --check; then
    dir="$("$_KIT_BIN_DIR/project-root.sh")/.claude/$kind"
    if ((create)) && [[ "$kind" == scratch ]]; then
      local registry="$KIT_LOG_DIR/scratch-registry.txt"
      mkdir -p "$KIT_LOG_DIR"
      grep -qxF "$dir" "$registry" 2>/dev/null || echo "$dir" >>"$registry"
    fi
  elif [[ "$kind" == scratch ]]; then
    dir="$KIT_SCRATCH_HOME"
  else
    dir="$KIT_PLANS_HOME"
  fi

  if ((create)); then
    mkdir -p "$dir"
  fi
  printf '%s\n' "$dir"
}

# Strict mode plus a fail-open ERR trap (logs to stderr, exits 0) so a hook
# bug never blocks a legitimate tool call. Each hook sets HOOK_NAME first.
kit_hook_init() {
  set -euo pipefail
  trap 'echo "${HOOK_NAME:-hook}: unexpected error, failing open" >&2; exit 0' ERR
}

# Reads stdin into the global $payload; each hook reads stdin exactly once.
read_payload() {
  payload="$(cat)"
}

# Fails open (allow) if jq is missing - without it a hook cannot safely
# evaluate policy.
require_jq() {
  if ! command -v jq >/dev/null 2>&1; then
    echo "${HOOK_NAME:-hook}: jq not found, skipping checks" >&2
    exit 0
  fi
}

extract_path() {
  printf '%s' "$payload" | jq -r '
    .tool_input.file_path
    // .tool_input.path
    // .tool_input.target_file
    // empty
  '
}

extract_command() {
  printf '%s' "$payload" | jq -r '.tool_input.command // empty'
}

# log_event <log> <event> [key value]...
# Appends one line to $KIT_LOG_DIR/<log>.jsonl: ts, hook, event, the
# payload's session_id, then each key/value pair, an empty value as null.
# Never fails its caller: a log that can't be written is skipped.
log_event() {
  local log="$1" event="$2" session_id=""
  shift 2
  local -a pairs=()
  while (($# >= 2)); do
    pairs+=(--arg "$1" "$2")
    shift 2
  done
  command -v jq >/dev/null 2>&1 || return 0
  session_id="$(printf '%s' "${payload:-}" | jq -r '.session_id // ""' 2>/dev/null || true)"
  mkdir -p "$KIT_LOG_DIR" 2>/dev/null || return 0
  jq -cn \
    --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg hook "${HOOK_NAME:-}" \
    --arg event "$event" \
    --arg session_id "$session_id" \
    "${pairs[@]}" \
    '$ARGS.named | map_values(if . == "" then null else . end)' \
    >>"$KIT_LOG_DIR/$log.jsonl" 2>/dev/null || true
}

# kit_args <words>
# Prints the non-option words of a command's argument string, one per line:
# words starting with "-" are skipped until a "--", after which every word
# counts. Split with read, so no word is ever glob-expanded against the
# caller's cwd. Options that take a separate value word aren't known here;
# callers needing those walk the words themselves.
kit_args() {
  local word opts_done=0
  local -a words
  read -ra words <<<"$1"
  for word in "${words[@]}"; do
    if ((! opts_done)); then
      case "$word" in
      --)
        opts_done=1
        continue
        ;;
      -*) continue ;;
      esac
    fi
    printf '%s\n' "$word"
  done
}

# True when rule slug $1 is listed in kit.yml's disabled_rules.
kit_rule_disabled() {
  kit_stacks_load
  local rule
  for rule in "${KIT_DISABLED_RULES[@]}"; do
    [[ "$rule" == "$1" ]] && return 0
  done
  return 1
}

# block <reason> <rule-slug> [context]
# Blocks the tool call (exit 2, reason on stderr) and logs the rule to
# guards.jsonl. A rule listed in disabled_rules is logged as disabled and
# returns instead, so the caller carries on as if it hadn't matched.
# context defaults to $KIT_BLOCK_CONTEXT, which a hook sets once, such as
# "Command: <cmd>".
block() {
  local reason="$1" rule="${2:-}" context="${3:-${KIT_BLOCK_CONTEXT:-}}"
  if [[ -n "$rule" ]] && kit_rule_disabled "$rule"; then
    log_event guards disabled rule "$rule"
    return 0
  fi
  log_event guards block rule "$rule"
  echo "Blocked by ${HOOK_NAME:-hook}: $reason" >&2
  if [[ -n "$context" ]]; then
    echo "$context" >&2
  fi
  exit 2
}

# Longest run of consecutive non-blank, non-list/heading/blockquote/table
# lines in $1, outside fenced code blocks and outside YAML frontmatter - a
# deterministic stand-in for rules/output.md section 1 (no walls of text);
# the rest of that rule needs judgment a hook can't make.
#
# Frontmatter is skipped because its delimiters and key: value lines match no
# skip pattern: a 5-key block reads as a 7-line wall, so every agent and skill
# definition would block on its own header.
longest_prose_run() {
  printf '%s\n' "$1" | awk '
    NR == 1 && /^---[[:space:]]*$/ { infm = 1; next }
    infm && /^---[[:space:]]*$/ { infm = 0; next }
    infm { next }
    /^```/ { infence = !infence; next }
    infence { next }
    NF == 0 { run = 0; next }
    /^[[:space:]]*([0-9]+[.)]|[-*+][[:space:]]|#{1,6}[[:space:]]|>|\|)/ { run = 0; next }
    { run++; if (run > best) best = run }
    END { print best + 0 }
  '
}

# Every derived list below comes from one yq pass, cached as sourceable
# `declare -p` output for as long as kit.yml is unchanged. Sourcing this
# file used to spawn five yq processes (~59ms), paid by guard-bash.sh on
# every Bash tool call and by project-root.sh on every bin script.
#
# STACK_SENTINELS_FULL: every sentinel any consumer cares about - used by
# inject-context.sh for cache invalidation and by detect-stack.sh as the
# canonical union. STACK_SENTINEL_STACKS[i] is the stack that declares
# STACK_SENTINELS_FULL[i] (kit_dir_has_stack).
# STACK_SENTINELS_PROJECT_ROOT: the anchor: true subset project-root.sh walks
# ancestors with. Keep minimal; every entry slows the walk for repos without
# that sentinel.
# STACK_DETECT_FILES: union of every file that influences stack detection -
# sentinels, every path referenced in extras rules (file:, in:, any_of[].file,
# any_of[].in[]), and every package_managers lockfile, so adding tsconfig.json
# or conftest.py, or switching a project from npm's lockfile to
# pnpm-lock.yaml, triggers re-detection. Without the lockfiles a switched
# project kept advertising its old [pm] tag in <repo-context> while
# resolve_package_manager (which stats lockfiles live) had already moved on.
# Known limit: only $project_root/<file> is checked, not subprojects, so an
# edit inside a subproject waits for a root file change to re-detect.
# STACK_PM_LOCKFILES / STACK_PM_MANAGERS / STACK_PM_ECOSYSTEMS: positional
# triples from .package_managers, walked by resolve_package_manager and by
# guard-bash.sh's per-ecosystem PM mismatch guard.
# KIT_GUARDED_LOCKFILES: package_managers lockfiles not marked hand_edited,
# plus extra_lockfiles; guard-edit.sh blocks direct edits to them.
# KIT_PROTECTED_BRANCHES, KIT_RC_FILES, KIT_SENSITIVE_PATHS, KIT_LOG_MAX_LINES,
# KIT_DISABLED_RULES: the guard lists of the same names in kit.yml.
# KIT_TP_*, KIT_CHECK_*, KIT_TC_*: positional columns of task_providers,
# checks, and toolchain_checks. "-" marks an empty column, since the row
# reader drops empty values and would shift every later row.
# KIT_SUBPROJECT_MAX_DEPTH: subproject_max_depth.
# KIT_ORCH_*: positional columns of orchestrators (task_paths space-joined).
# KIT_TOOLS: tools.
# KIT_FMT_*: positional columns of formatters; KIT_DISABLED_FORMATTERS.
_stacks_lists_cache="$KIT_CACHE_DIR/stacks-lists.$KIT_YML_TAG.bash"

# Bump on every change to the queries below or to the cache's shape. The
# mtime check only sees kit.yml, so without this an existing cache
# outlives a rewritten derivation and keeps serving the old lists.
_stacks_lists_format=10

_reset_stacks_lists() {
  STACK_SENTINELS_FULL=()
  STACK_SENTINEL_STACKS=()
  STACK_SENTINELS_PROJECT_ROOT=()
  STACK_DETECT_FILES=()
  STACK_PM_LOCKFILES=()
  STACK_PM_MANAGERS=()
  STACK_PM_ECOSYSTEMS=()
  KIT_GUARDED_LOCKFILES=()
  KIT_PROTECTED_BRANCHES=()
  KIT_RC_FILES=()
  KIT_SENSITIVE_PATHS=()
  KIT_DISABLED_RULES=()
  KIT_LOG_MAX_LINES=10000
  KIT_TP_NAMES=()
  KIT_TP_STACKS=()
  KIT_TP_MANIFESTS=()
  KIT_TP_EXTRACTORS=()
  KIT_TP_ARGS=()
  KIT_TP_RUNS=()
  KIT_TP_RUNS_BY_PM=()
  KIT_CHECK_NAMES=()
  KIT_CHECK_TASKS=()
  KIT_CHECK_EXCLUDES=()
  KIT_TC_STACKS=()
  KIT_TC_NAMES=()
  KIT_TC_CMDS=()
  KIT_TC_BINS=()
  KIT_TC_WHEN_DIRS=()
  KIT_SUBPROJECT_MAX_DEPTH=4
  KIT_ORCH_NAMES=()
  KIT_ORCH_SIGNALS=()
  KIT_ORCH_TASK_PATHS=()
  KIT_ORCH_RUNS=()
  KIT_TOOLS=()
  KIT_FMT_NAMES=()
  KIT_FMT_EXTS=()
  KIT_FMT_FILES=()
  KIT_FMT_TOMLS=()
  KIT_FMT_PRETTIER=()
  KIT_FMT_BINS=()
  KIT_FMT_CMDS=()
  KIT_DISABLED_FORMATTERS=()
}

# Each emitted row is "<list-tag>\t<value>" so one yq call fills every list.
_build_stacks_lists() {
  local kind value
  _reset_stacks_lists

  while IFS=$'\t' read -r kind value; do
    [[ -z "$value" || "$value" == "null" ]] && continue
    case "$kind" in
    FULL) STACK_SENTINELS_FULL+=("$value") ;;
    SENT_STACK) STACK_SENTINEL_STACKS+=("$value") ;;
    ANCHOR) STACK_SENTINELS_PROJECT_ROOT+=("$value") ;;
    DETECT) STACK_DETECT_FILES+=("$value") ;;
    LOCKFILE) STACK_PM_LOCKFILES+=("$value") ;;
    MANAGER) STACK_PM_MANAGERS+=("$value") ;;
    ECOSYSTEM) STACK_PM_ECOSYSTEMS+=("$value") ;;
    GUARDLOCK) KIT_GUARDED_LOCKFILES+=("$value") ;;
    PROTECTED) KIT_PROTECTED_BRANCHES+=("$value") ;;
    RC) KIT_RC_FILES+=("$value") ;;
    SENSITIVE) KIT_SENSITIVE_PATHS+=("$value") ;;
    DISABLED) KIT_DISABLED_RULES+=("$value") ;;
    TP_NAME) KIT_TP_NAMES+=("$value") ;;
    TP_STACK) KIT_TP_STACKS+=("$value") ;;
    TP_MANIFESTS) KIT_TP_MANIFESTS+=("$value") ;;
    TP_EXTRACTOR) KIT_TP_EXTRACTORS+=("$value") ;;
    TP_ARG) KIT_TP_ARGS+=("$value") ;;
    TP_RUN) KIT_TP_RUNS+=("$value") ;;
    TP_RUNPM) KIT_TP_RUNS_BY_PM+=("$value") ;;
    CHECK_NAME) KIT_CHECK_NAMES+=("$value") ;;
    CHECK_TASKS) KIT_CHECK_TASKS+=("$value") ;;
    CHECK_EXCLUDE) KIT_CHECK_EXCLUDES+=("$value") ;;
    TC_STACK) KIT_TC_STACKS+=("$value") ;;
    TC_NAME) KIT_TC_NAMES+=("$value") ;;
    TC_CMD) KIT_TC_CMDS+=("$value") ;;
    TC_BIN) KIT_TC_BINS+=("$value") ;;
    TC_WHEN_DIR) KIT_TC_WHEN_DIRS+=("$value") ;;
    SUBDEPTH) KIT_SUBPROJECT_MAX_DEPTH="$value" ;;
    ORCH_NAME) KIT_ORCH_NAMES+=("$value") ;;
    ORCH_SIGNAL) KIT_ORCH_SIGNALS+=("$value") ;;
    ORCH_PATHS) KIT_ORCH_TASK_PATHS+=("$value") ;;
    ORCH_RUN) KIT_ORCH_RUNS+=("$value") ;;
    TOOL) KIT_TOOLS+=("$value") ;;
    FMT_NAME) KIT_FMT_NAMES+=("$value") ;;
    FMT_EXTS) KIT_FMT_EXTS+=("$value") ;;
    FMT_FILES) KIT_FMT_FILES+=("$value") ;;
    FMT_TOML) KIT_FMT_TOMLS+=("$value") ;;
    FMT_PRETTIER) KIT_FMT_PRETTIER+=("$value") ;;
    FMT_BIN) KIT_FMT_BINS+=("$value") ;;
    FMT_CMD) KIT_FMT_CMDS+=("$value") ;;
    FMT_DISABLED) KIT_DISABLED_FORMATTERS+=("$value") ;;
    LOGMAX) KIT_LOG_MAX_LINES="$value" ;;
    esac
  done < <(
    # shellcheck disable=SC2016  # $stack is a yq variable
    yq -r '
      [
        (.stacks[].sentinels[] | ["FULL", .name]),
        (.stacks | to_entries[] | .key as $stack | .value.sentinels[] | ["SENT_STACK", $stack]),
        (.stacks[].sentinels[] | select(.anchor == true) | ["ANCHOR", .name]),
        (.stacks[].sentinels[] | ["DETECT", .name]),
        (.stacks[].extras[]? | select(has("file")) | ["DETECT", .file]),
        (.stacks[].extras[]? | select(has("in")) | .in[] | ["DETECT", .]),
        (.stacks[].extras[]? | .any_of // [] | .[] | select(has("file")) | ["DETECT", .file]),
        (.stacks[].extras[]? | .any_of // [] | .[] | select(has("in")) | .in[] | ["DETECT", .]),
        (.package_managers[] | ["DETECT", .lockfile]),
        (.package_managers[] | ["LOCKFILE", .lockfile]),
        (.package_managers[] | ["MANAGER", .manager]),
        (.package_managers[] | ["ECOSYSTEM", .ecosystem // "none"]),
        (.package_managers[] | select(.hand_edited != true) | ["GUARDLOCK", .lockfile]),
        (.extra_lockfiles // [] | .[] | ["GUARDLOCK", .]),
        (.protected_branches // [] | .[] | ["PROTECTED", .]),
        (.rc_files // [] | .[] | ["RC", .]),
        (.sensitive_paths // [] | .[] | ["SENSITIVE", .]),
        (.disabled_rules // [] | .[] | ["DISABLED", .]),
        (.log_max_lines // 10000 | ["LOGMAX", .]),
        (.subproject_max_depth // 4 | ["SUBDEPTH", .]),
        (.task_providers // [] | .[] | (
          ["TP_NAME", .name],
          ["TP_STACK", (.stack // "-")],
          ["TP_MANIFESTS", ((.manifests // []) | join(" ") | select(. != "") // "-")],
          ["TP_EXTRACTOR", .extractor],
          ["TP_ARG", (.arg // "-")],
          ["TP_RUN", .run],
          ["TP_RUNPM", ((.run_by_pm // {}) | to_entries | map(.key + "=" + .value) | join(";") | select(. != "") // "-")]
        )),
        (.checks // [] | .[] | (
          ["CHECK_NAME", .name],
          ["CHECK_TASKS", ((.tasks // []) | join(" "))],
          ["CHECK_EXCLUDE", ((.exclude // []) | join(" ") | select(. != "") // "-")]
        )),
        (.tools // [] | .[] | ["TOOL", .]),
        (.formatters // [] | .[] | (
          ["FMT_NAME", .name],
          ["FMT_EXTS", ((.ext // []) | join(" "))],
          ["FMT_FILES", ((.signal_files // []) | join(" ") | select(. != "") // "-")],
          ["FMT_TOML", (.signal_toml // "-")],
          ["FMT_PRETTIER", ((.signal_prettier // false) | tostring)],
          ["FMT_BIN", .bin],
          ["FMT_CMD", .cmd]
        )),
        (.disabled_formatters // [] | .[] | ["FMT_DISABLED", .]),
        (.orchestrators // [] | .[] | (
          ["ORCH_NAME", .name],
          ["ORCH_SIGNAL", .signal],
          ["ORCH_PATHS", ((.task_paths // []) | join(" "))],
          ["ORCH_RUN", .run]
        )),
        (.toolchain_checks // [] | .[] | (
          ["TC_STACK", .stack],
          ["TC_NAME", .name],
          ["TC_CMD", .cmd],
          ["TC_BIN", ((.bin // []) | join(" ") | select(. != "") // "-")],
          ["TC_WHEN_DIR", (.when_dir // "-")]
        ))
      ] | .[] | join("\t")
    ' "$KIT_YML" 2>/dev/null
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

# Fills the STACK_* arrays once per process, from the cache when it is fresh.
# Every function below that reads one calls this first; so does any caller
# reading an array directly.
_kit_stacks_loaded=0
kit_stacks_load() {
  ((_kit_stacks_loaded)) && return 0
  _kit_stacks_loaded=1

  # A fresh cache needs no yq, so the guard lists survive yq going missing.
  _stacks_lists_cached_format=""
  if [[ -s "$_stacks_lists_cache" && "$_stacks_lists_cache" -nt "$KIT_YML" ]]; then
    # shellcheck disable=SC1090
    source "$_stacks_lists_cache"
  fi
  [[ "$_stacks_lists_cached_format" == "$_stacks_lists_format" ]] && return 0

  if ! command -v yq >/dev/null 2>&1; then
    echo "_lib.sh: yq not found; stack detection and kit.yml guard lists disabled" >&2
    _reset_stacks_lists
    return 0
  fi

  _build_stacks_lists
  _stacks_lists_cached_format="$_stacks_lists_format"
  # The temp file goes in the cache dir, not TMPDIR: mv is only atomic
  # within one filesystem, and a half-written cache is sourceable garbage.
  ((${#STACK_SENTINELS_FULL[@]} > 0)) || return 0
  mkdir -p "$(dirname "$_stacks_lists_cache")" 2>/dev/null || true
  local tmp
  tmp="$(mktemp "$(dirname "$_stacks_lists_cache")/.stacks-lists.XXXXXX" 2>/dev/null || true)"
  [[ -n "$tmp" ]] || return 0
  # -g: the cache is sourced from inside this function, which would
  # otherwise scope every array to it and hand the caller empty lists.
  if declare -p STACK_SENTINELS_FULL STACK_SENTINEL_STACKS STACK_SENTINELS_PROJECT_ROOT \
    STACK_DETECT_FILES STACK_PM_LOCKFILES STACK_PM_MANAGERS STACK_PM_ECOSYSTEMS \
    KIT_GUARDED_LOCKFILES KIT_PROTECTED_BRANCHES KIT_RC_FILES KIT_SENSITIVE_PATHS \
    KIT_DISABLED_RULES KIT_LOG_MAX_LINES KIT_SUBPROJECT_MAX_DEPTH \
    KIT_TP_NAMES KIT_TP_STACKS KIT_TP_MANIFESTS KIT_TP_EXTRACTORS KIT_TP_ARGS \
    KIT_TP_RUNS KIT_TP_RUNS_BY_PM KIT_CHECK_NAMES KIT_CHECK_TASKS KIT_CHECK_EXCLUDES \
    KIT_TC_STACKS KIT_TC_NAMES KIT_TC_CMDS KIT_TC_BINS KIT_TC_WHEN_DIRS \
    KIT_ORCH_NAMES KIT_ORCH_SIGNALS KIT_ORCH_TASK_PATHS KIT_ORCH_RUNS KIT_TOOLS \
    KIT_FMT_NAMES KIT_FMT_EXTS KIT_FMT_FILES KIT_FMT_TOMLS KIT_FMT_PRETTIER \
    KIT_FMT_BINS KIT_FMT_CMDS KIT_DISABLED_FORMATTERS \
    _stacks_lists_cached_format |
    sed -E -e 's/^declare -- /declare -g /' -e 's/^declare -([aA])/declare -g\1/' \
      >"$tmp" 2>/dev/null; then
    mv "$tmp" "$_stacks_lists_cache" 2>/dev/null || rm -f "$tmp"
  else
    rm -f "$tmp"
  fi
}

# True when path $1 (a leading ~, $HOME or ${HOME} allowed) is a credential
# or key file per kit.yml's sensitive_paths.
kit_is_sensitive_path() {
  kit_stacks_load
  local path="$1" entry home_path
  # A shell word may still carry its quotes: cat "$HOME/.ssh/id_rsa".
  path="${path#[\"\']}"
  path="${path%[\"\']}"
  path="${path/#\~/$HOME}"
  path="${path/#\$HOME/$HOME}"
  path="${path/#\$\{HOME\}/$HOME}"
  for entry in "${KIT_SENSITIVE_PATHS[@]}"; do
    if [[ "$entry" == \~/* ]]; then
      home_path="$HOME/${entry#\~/}"
      if [[ "$home_path" == */ ]]; then
        [[ "$path" == "$home_path"* ]] && return 0
      else
        [[ "$path" == "$home_path" ]] && return 0
      fi
    else
      # shellcheck disable=SC2053  # a basename glob on purpose
      [[ "${path##*/}" == $entry ]] && return 0
    fi
  done
  return 1
}

# True when path $1 (same expansions) is a shell rc file per rc_files.
kit_is_rc_file() {
  kit_stacks_load
  local path="$1" entry
  path="${path/#\~/$HOME}"
  path="${path/#\$HOME/$HOME}"
  path="${path/#\$\{HOME\}/$HOME}"
  for entry in "${KIT_RC_FILES[@]}"; do
    [[ "$path" == "$HOME/${entry#\~/}" ]] && return 0
  done
  return 1
}

# True when the basename of $1 is a tool-generated lockfile.
kit_is_guarded_lockfile() {
  kit_stacks_load
  local entry
  for entry in "${KIT_GUARDED_LOCKFILES[@]}"; do
    [[ "${1##*/}" == "$entry" ]] && return 0
  done
  return 1
}

# resolve_package_manager <dir>
# Prints the package manager name for <dir> by walking the package_managers
# table in kit.yml (first lockfile match wins). Checks <dir> first, then
# the git toplevel of <dir> to handle monorepos where lockfiles live at the
# root. Prints nothing when no lockfile is found; callers should apply their
# own default (e.g. npm) when empty output means "no preference".
resolve_package_manager() {
  kit_stacks_load
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

# Extractors: each reads one file, prints one item per line, and prints
# nothing when the file or the path in it is missing. A dotted path
# (".scripts", ".tool.poe.tasks") becomes a jq getpath() array, so config
# never becomes filter code.
# shellcheck disable=SC2016  # $p is a jq variable
_KIT_GETPATH='getpath($p | ltrimstr(".") | split(".") | map(select(. != "")))'

# Keys, or string items, at dotted path $1 of the JSON on stdin.
_kit_stdin_keys() {
  jq -r --arg p "$1" "$_KIT_GETPATH"' | if type == "object" then keys_unsorted[] else empty end' 2>/dev/null || true
}
_kit_stdin_array() {
  jq -r --arg p "$1" "$_KIT_GETPATH"' | if type == "array" then .[] | strings else empty end' 2>/dev/null || true
}

# json_keys <file> <path>: keys of the object at <path>, in file order.
# json_array <file> <path>: string items of the array at <path>.
# toml_keys, toml_array, yaml_array: the same, reading TOML or YAML via yq.
json_keys() {
  [[ -f "$1" ]] || return 0
  _kit_stdin_keys "$2" <"$1"
}
json_array() {
  [[ -f "$1" ]] || return 0
  _kit_stdin_array "$2" <"$1"
}
toml_keys() {
  [[ -f "$1" ]] || return 0
  yq -p toml -o json '.' "$1" 2>/dev/null | _kit_stdin_keys "$2"
}
toml_array() {
  [[ -f "$1" ]] || return 0
  yq -p toml -o json '.' "$1" 2>/dev/null | _kit_stdin_array "$2"
}
yaml_array() {
  [[ -f "$1" ]] || return 0
  yq -o json '.' "$1" 2>/dev/null | _kit_stdin_array "$2"
}

# json_value <file> <path>: the string or number at <path>.
json_value() {
  [[ -f "$1" ]] || return 0
  jq -r --arg p "$2" "$_KIT_GETPATH"' | if type == "string" or type == "number" then . else empty end' "$1" 2>/dev/null || true
}

# toml_package_version <lockfile> <name>: the version of [[package]] <name>
# in a uv.lock or poetry.lock.
toml_package_version() {
  [[ -f "$1" ]] || return 0
  yq -p toml -o json '.' "$1" 2>/dev/null |
    jq -r --arg n "$2" 'first(.package[]? | select((.name | ascii_downcase) == ($n | ascii_downcase)) | .version) // empty' 2>/dev/null || true
}

# toml_has <file> <path>: true when <path> exists in the TOML file, even as an
# empty table (a bare [tool.ruff] means "use ruff with defaults").
toml_has() {
  [[ -f "$1" ]] || return 1
  yq -p toml -o json '.' "$1" 2>/dev/null |
    jq -e --arg p "$2" "$_KIT_GETPATH"' != null' >/dev/null 2>&1
}

# find_up <start> <stop> <name>...
# Prints the first <start>/<name>, then its parent's, walking up to and
# including <stop> and never above it. Nothing when none exists.
find_up() {
  local dir="$1" stop="$2" name
  shift 2
  while :; do
    for name in "$@"; do
      if [[ -e "$dir/$name" ]]; then
        printf '%s\n' "$dir/$name"
        return 0
      fi
    done
    [[ "$dir" == "$stop" || "$dir" == / || "$dir" != "$stop"/* ]] && return 0
    dir="${dir%/*}"
    [[ -n "$dir" ]] || dir=/
  done
}

# make_targets <file>: explicit targets, in file order. Skips special
# (.PHONY), pattern (%), and variable-assignment (:=) lines.
make_targets() {
  [[ -f "$1" ]] || return 0
  grep -E '^[A-Za-z0-9][A-Za-z0-9_.-]*[[:space:]]*:([^=]|$)' "$1" 2>/dev/null |
    sed -E 's/[[:space:]]*:.*//' | awk '!seen[$0]++' || true
}

# just_recipes <file>: recipe names, from `just --summary` when just is
# installed, else from recipe header lines.
just_recipes() {
  [[ -f "$1" ]] || return 0
  if command -v just >/dev/null 2>&1; then
    just --justfile "$1" --summary 2>/dev/null | tr ' ' '\n' | awk 'NF' || true
    return 0
  fi
  grep -E '^@?[A-Za-z_][A-Za-z0-9_-]*([[:space:]][^:=]*)?:([^=]|$)' "$1" 2>/dev/null |
    sed -E 's/^@?([A-Za-z0-9_-]+).*/\1/' | awk '!seen[$0]++' || true
}

# regex_lines <file> <ERE>: for each matching line, the last capture group.
regex_lines() {
  [[ -f "$1" ]] || return 0
  local line
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ "$line" =~ $2 ]] || continue
    printf '%s\n' "${BASH_REMATCH[${#BASH_REMATCH[@]} - 1]}"
  done <"$1"
}

# Runs extractor $1 on file $2 with arg $3. Only names in this list run.
_kit_extract() {
  case "$1" in
  json_keys | json_array | json_value | toml_keys | toml_array | yaml_array | toml_package_version | regex_lines) "$1" "$2" "$3" ;;
  make_targets | just_recipes) "$1" "$2" ;;
  *) echo "_lib.sh: unknown extractor in kit.yml: $1" >&2 ;;
  esac
}

# kit_nearest_pm_lockfile <dir> <ecosystem>
# Prints "manager:lockfile" for the nearest lockfile of <ecosystem>, walking
# from <dir> up to its git toplevel (only <dir> outside a repo). Nothing when
# that ecosystem has no lockfile on the way: greenfield. Unlike
# resolve_package_manager, a pnpm root never answers for a uv service below it.
# <dir> must be physical to meet the toplevel; callers resolve it.
kit_nearest_pm_lockfile() {
  kit_stacks_load
  local dir="$1" eco="$2" top i
  top="$(git -C "$dir" rev-parse --show-toplevel 2>/dev/null || true)"
  while :; do
    for ((i = 0; i < ${#STACK_PM_LOCKFILES[@]}; i++)); do
      [[ "${STACK_PM_ECOSYSTEMS[i]:-}" == "$eco" ]] || continue
      if [[ -f "$dir/${STACK_PM_LOCKFILES[i]}" ]]; then
        printf '%s:%s\n' "${STACK_PM_MANAGERS[i]}" "${STACK_PM_LOCKFILES[i]}"
        return 0
      fi
    done
    if [[ -z "$top" || "$dir" == "$top" || "$dir" == / ]]; then
      return 0
    fi
    dir="${dir%/*}"
    [[ -n "$dir" ]] || dir=/
  done
}

# kit_toolchain_cmd <index> <dir>
# Resolves toolchain_checks entry <index> for <dir>: sets KIT_TC_CMD with
# {bin} filled in, or returns 1 with KIT_TC_SKIP saying why it can't run
# (its when_dir is missing, or none of its bins is on PATH). run-checks.sh
# and the <tooling> block both use it, so Claude is never told about a
# check that the checks skip.
# shellcheck disable=SC2034  # KIT_TC_SKIP is read by the callers
kit_toolchain_cmd() {
  local i="$1" dir="$2" bin found=""
  local -a bins
  KIT_TC_CMD="" KIT_TC_SKIP=""
  if [[ "${KIT_TC_WHEN_DIRS[i]}" != - && ! -d "$dir/${KIT_TC_WHEN_DIRS[i]}" ]]; then
    KIT_TC_SKIP="no ${KIT_TC_WHEN_DIRS[i]}/ yet"
    return 1
  fi
  KIT_TC_CMD="${KIT_TC_CMDS[i]}"
  [[ "${KIT_TC_BINS[i]}" != - ]] || return 0
  read -ra bins <<<"${KIT_TC_BINS[i]}"
  for bin in "${bins[@]}"; do
    if command -v "$bin" >/dev/null 2>&1; then
      found="$bin"
      break
    fi
  done
  if [[ -z "$found" ]]; then
    KIT_TC_SKIP="none of ${KIT_TC_BINS[i]} on PATH"
    return 1
  fi
  KIT_TC_CMD="${KIT_TC_CMD//\{bin\}/$found}"
}

# True when dir $1 holds a sentinel of stack $2.
kit_dir_has_stack() {
  kit_stacks_load
  local i
  for ((i = 0; i < ${#STACK_SENTINELS_FULL[@]}; i++)); do
    if [[ "${STACK_SENTINEL_STACKS[i]:-}" == "$2" && -f "$1/${STACK_SENTINELS_FULL[i]}" ]]; then
      return 0
    fi
  done
  return 1
}

# Prints the manifest path task provider index $1 reads in dir $2, or nothing
# when none of its manifests is there.
_kit_provider_manifest() {
  local manifest
  local -a manifests
  read -ra manifests <<<"${KIT_TP_MANIFESTS[$1]}"
  for manifest in "${manifests[@]}"; do
    if [[ -f "$2/$manifest" ]]; then
      printf '%s\n' "$2/$manifest"
      return 0
    fi
  done
}

# kit_providers [dir]
# Prints "<provider>\t<stack>" for every task_providers entry whose manifest
# is in <dir>, tasks or not ("-" when a provider has no stack).
kit_providers() {
  kit_stacks_load
  local dir="${1:-.}" i
  for ((i = 0; i < ${#KIT_TP_NAMES[@]}; i++)); do
    if [[ -n "$(_kit_provider_manifest "$i" "$dir")" ]]; then
      printf '%s\t%s\n' "${KIT_TP_NAMES[i]}" "${KIT_TP_STACKS[i]}"
    fi
  done
}

# kit_tasks [dir]
# Prints "<provider>\t<stack>\t<task>\t<command>" for every task of every
# task_providers entry whose manifest is in <dir> ("-" when a provider has no
# stack). {pm} is the nearest lockfile's manager for the provider's stack
# (kit_nearest_pm_lockfile), npm when there is none.
kit_tasks() {
  kit_stacks_load
  local dir="${1:-.}" i file run pm_run task pm eco phys=""
  local -a pm_runs
  # Per provider stack, nearest lockfile first: a poetry service under a
  # pnpm root runs poe through poetry, and a subproject's own lockfile beats
  # the root's.
  local -A pm_by_eco=()
  for ((i = 0; i < ${#KIT_TP_NAMES[@]}; i++)); do
    file="$(_kit_provider_manifest "$i" "$dir")"
    [[ -n "$file" ]] || continue

    eco="${KIT_TP_STACKS[i]}"
    if [[ -z "${pm_by_eco[$eco]+set}" ]]; then
      [[ -n "$phys" ]] || phys="$(cd -P "$dir" 2>/dev/null && pwd)"
      pm_by_eco[$eco]=""
      if [[ "$eco" != - && -n "$phys" ]]; then
        pm_by_eco[$eco]="$(kit_nearest_pm_lockfile "$phys" "$eco")"
        pm_by_eco[$eco]="${pm_by_eco[$eco]%%:*}"
      fi
    fi
    pm="${pm_by_eco[$eco]}"
    run="${KIT_TP_RUNS[i]}"
    if [[ "${KIT_TP_RUNS_BY_PM[i]}" != - ]]; then
      IFS=';' read -ra pm_runs <<<"${KIT_TP_RUNS_BY_PM[i]}"
      for pm_run in "${pm_runs[@]}"; do
        if [[ "${pm_run%%=*}" == "$pm" ]]; then
          run="${pm_run#*=}"
        fi
      done
    fi
    run="${run//\{pm\}/${pm:-npm}}"

    while IFS= read -r task; do
      [[ -n "$task" ]] || continue
      printf '%s\t%s\t%s\t%s\n' "${KIT_TP_NAMES[i]}" "${KIT_TP_STACKS[i]}" "$task" "${run//\{task\}/$task}"
    done < <(_kit_extract "${KIT_TP_EXTRACTORS[i]}" "$file" "${KIT_TP_ARGS[i]}")
  done
}

# kit_subprojects [root]
# Prints "." (the root), then every subproject directory relative to it,
# sorted: each directory holding a tracked anchor sentinel at most
# subproject_max_depth levels down, and each member its workspace manifests
# name (package.json workspaces, pnpm-workspace.yaml, Cargo, go.work, uv).
# Tracked files only, so node_modules and virtualenvs never count. Root
# defaults to the git toplevel, else $PWD.
kit_subprojects() {
  kit_stacks_load
  local root="${1:-}"
  [[ -n "$root" ]] || root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
  local -a pathspecs=()
  local sentinel
  for sentinel in "${STACK_SENTINELS_PROJECT_ROOT[@]}"; do
    pathspecs+=(":(glob)**/$sentinel")
  done

  printf '.\n'
  {
    if ((${#pathspecs[@]} > 0)); then
      local path dir slashes
      git -C "$root" ls-files -- "${pathspecs[@]}" 2>/dev/null | while IFS= read -r path; do
        [[ "$path" == */* ]] || continue
        dir="${path%/*}"
        slashes="${dir//[^\/]/}"
        if ((${#slashes} + 1 <= KIT_SUBPROJECT_MAX_DEPTH)); then
          printf '%s\n' "$dir"
        fi
      done
    fi
    {
      json_array "$root/package.json" .workspaces
      json_array "$root/package.json" .workspaces.packages
      yaml_array "$root/pnpm-workspace.yaml" .packages
      toml_array "$root/Cargo.toml" .workspace.members
      toml_array "$root/pyproject.toml" .tool.uv.workspace.members
      regex_lines "$root/go.work" '^[[:space:]]*(use[[:space:]]+)?\(?[[:space:]]*(\.[^[:space:]()]*)'
    } | while IFS= read -r pattern; do
      # Negated entries only narrow a pnpm glob; nothing to add.
      [[ "$pattern" == '!'* ]] && continue
      pattern="${pattern#./}"
      (
        cd "$root" 2>/dev/null || exit 0
        shopt -s globstar nullglob
        # Unquoted on purpose: the workspace pattern is a glob to expand.
        # shellcheck disable=SC2086
        for dir in $pattern; do
          if [[ -d "$dir" ]]; then
            printf '%s\n' "${dir%/}"
          fi
        done
      )
    done
  } | sed -e 's|^\./||' -e 's|/$||' | awk 'NF && $0 != "." && !seen[$0]++' | sort
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
  printf '%s/%s-%s.%s.txt\n' \
    "$KIT_CACHE_DIR/stack" \
    "$project_name" \
    "$(printf '%s' "$project_root" | shasum -a 256 | cut -c1-8)" \
    "$KIT_YML_TAG"
}

# refresh_stack_cache_if_stale <project_root> <cache_file>
# Regenerates the cache by running detect-stack.sh when any detection-relevant
# file (STACK_DETECT_FILES) is newer than the cache, or the cache is
# empty/missing. No output; callers read $cache_file afterward.
#
# kit.yml counts as detection-relevant: it defines what detect-stack.sh
# looks for, so adding a stack, sentinel or extra must re-detect every
# project. Without it a cached project kept serving the old detection until
# one of its own sentinel files happened to be touched, while the
# stacks-lists and skill-map caches had already moved on.
refresh_stack_cache_if_stale() {
  kit_stacks_load
  local project_root="$1" cache_file="$2"
  mkdir -p "$(dirname "$cache_file")"

  # -L on both stat forms: $KIT_YML can be the ~/.claude symlink into the
  # dotfiles repo, and an undereferenced stat reports the link's own mtime,
  # which never changes when the file behind it is edited.
  local newest_sentinel=0 f m
  for f in "${STACK_DETECT_FILES[@]/#/$project_root/}" "$KIT_YML"; do
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
    [[ "$line" =~ ^(root|versions)[:\ ] ]] && continue
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
# Prints kit.yml's global_skills, deduped, in first-seen order.
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
# kit.yml, deduped, first-seen order, excluding global_skills (those are
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
