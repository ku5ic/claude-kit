#!/usr/bin/env bash
# PreToolUse hook for Edit, Write, MultiEdit: blocks writes to risky paths
# regardless of permission rules. Callable standalone or sourced by
# guard-dispatch.sh, which reads the payload once for all three checks.
HOOK_NAME="guard-edit.sh"
# shellcheck source=_lib.sh
source "$(dirname "$0")/_lib.sh"

run_guard_edit() {
  # Shadows the file-scope HOOK_NAME via bash's dynamic scoping - without
  # this, sourcing all three guard files into one dispatcher process leaves
  # HOOK_NAME set to whichever file was sourced last, misattributing every
  # block() entry.
  local HOOK_NAME="guard-edit.sh"
  path="$(extract_path)"
  [[ -z "$path" ]] && return 0

  block() {
    echo "Blocked by ${HOOK_NAME}: $1" >&2
    echo "Path: $path" >&2
    exit 2
  }

  case "$(basename "$path")" in
  package-lock.json | pnpm-lock.yaml | yarn.lock | bun.lockb | Gemfile.lock | Cargo.lock | composer.lock | poetry.lock | uv.lock | pdm.lock | requirements.txt.lock)
    block "lockfile edit. Use the package manager." "lockfile-edit"
    ;;
  esac

  [[ "$path" =~ /\.git/ ]] && block "edit inside .git/" "git-dir-edit"

  case "$path" in
  "$HOME/.zshrc" | "$HOME/.zprofile" | "$HOME/.bashrc" | "$HOME/.bash_profile" | "$HOME/.profile")
    block "direct edit to a shell rc file. Use the dotfiles repo." "rc-edit"
    ;;
  esac

  # Credential and key paths are not repeated here: settings.json's deny
  # rules fire before any hook runs (verified 2026-09-12 by writing to a
  # scratch .env: the permission layer refused it and this hook never saw
  # the call). guard-bash.sh keeps its own copy because permissions cannot
  # express `cat ~/.ssh/id_rsa`.

  if [[ "$path" =~ \.github/workflows/.*\.ya?ml$ ]]; then
    echo "guard-edit: editing CI workflow $path" >&2
  fi

  return 0
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  read_payload
  require_jq
  run_guard_edit
fi
