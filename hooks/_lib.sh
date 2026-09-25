#!/usr/bin/env bash
# Shared helpers for ~/.claude/hooks/*.sh. Each hook sets HOOK_NAME and
# sources this file for strict mode plus a fail-open ERR trap (logs to
# stderr, exits 0, instead of blocking a legitimate tool call on a hook bug).

# Idempotency guard: guard-dispatch.sh sources this directly, then sources
# guard-edit.sh/guard-skills.sh which each source it again for standalone
# use - without this, the second sourcing reruns the `readonly` declarations
# below and errors.
[[ -n "${_CLAUDE_HOOKS_LIB_SOURCED:-}" ]] && return
_CLAUDE_HOOKS_LIB_SOURCED=1

set -euo pipefail
trap 'echo "${HOOK_NAME:-hook}: unexpected error, failing open" >&2; exit 0' ERR

# Kit root: the plugin root when installed as a plugin, else the parent of
# this hooks dir (~/.claude via symlinks). Tests point it at a fake tree.
# shellcheck disable=SC2034 # read by the hooks that source this file
KIT_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"

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

# $1 = human-readable reason, $2 = optional rule slug. Hooks override this
# to add context (e.g. the offending command or path).
block() {
  echo "Blocked by ${HOOK_NAME:-hook}: $1" >&2
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
