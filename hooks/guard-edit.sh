#!/usr/bin/env bash
# PreToolUse hook for Read, Edit, Write, MultiEdit: blocks reads and writes of
# credential files (kit.yml's sensitive_paths) and writes to other risky
# paths, regardless of permission rules. Plugins can't ship settings.json
# deny rules, so for a plugin install this is the only credential-read
# protection. Callable standalone or sourced by guard-dispatch.sh, which
# reads the payload once for every check.
HOOK_NAME="guard-edit.sh"
# shellcheck source=../bin/_lib.sh
source "$(dirname "$0")/../bin/_lib.sh"
kit_hook_init

run_guard_edit() {
  # Shadows the file-scope HOOK_NAME via bash's dynamic scoping - without
  # this, sourcing all three guard files into one dispatcher process leaves
  # HOOK_NAME set to whichever file was sourced last, misattributing every
  # block() entry.
  local HOOK_NAME="guard-edit.sh"
  path="$(extract_path)"
  [[ -z "$path" ]] && return 0
  local KIT_BLOCK_CONTEXT="Path: $path"

  local tool
  tool="$(printf '%s' "$payload" | jq -r '.tool_name // empty')"

  if kit_is_sensitive_path "$path"; then
    if [[ "$tool" == Read ]]; then
      block "reading a credential or key file is not permitted" "sensitive-read"
    else
      block "writing a credential or key file is not permitted" "sensitive-write"
    fi
  fi

  # Everything below guards writes only.
  [[ "$tool" == Read ]] && return 0

  if kit_is_guarded_lockfile "$path"; then
    block "lockfile edit. Use the package manager." "lockfile-edit"
  fi

  [[ "$path" =~ /\.git/ ]] && block "edit inside .git/" "git-dir-edit"

  if kit_is_rc_file "$path"; then
    block "direct edit to a shell rc file. Use the dotfiles repo." "rc-edit"
  fi

  if [[ "$path" =~ \.github/workflows/.*\.ya?ml$ ]]; then
    echo "guard-edit: editing CI workflow $path" >&2
  fi

  if kit_is_overlay_path "$path"; then
    emit_decision ask "this is the claude-kit overlay, which can switch the kit's own guards off; confirm the change"
  fi

  return 0
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  read_payload
  require_jq
  run_guard_edit
fi
