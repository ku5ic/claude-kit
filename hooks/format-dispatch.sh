#!/usr/bin/env bash
HOOK_NAME="format-dispatch.sh"
# shellcheck source=_lib.sh
source "$(dirname "$0")/_lib.sh"

read_payload
require_jq

path="$(extract_path)"
[[ -z "$path" || ! -f "$path" ]] && exit 0

case "$path" in
*/.claude/scratch/* | */scratch/*) exit 0 ;;
esac

case "$path" in
*.sh | *.bash)
  command -v shfmt >/dev/null 2>&1 && shfmt -i 2 -w "$path"
  command -v shellcheck >/dev/null 2>&1 && { shellcheck "$path" >&2 || true; }
  ;;
*.lua)
  command -v stylua >/dev/null 2>&1 && stylua "$path"
  ;;
*.js | *.jsx | *.ts | *.tsx | *.css | *.json | *.md)
  command -v prettier >/dev/null 2>&1 && prettier --write "$path" >/dev/null
  ;;
esac

exit 0
