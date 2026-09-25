#!/usr/bin/env bash
# PostToolUse hook for Write, Edit, MultiEdit: strips typographic punctuation
# (em dashes, smart quotes, etc.) from written files.
HOOK_NAME="sanitize-output.sh"
# shellcheck source=_lib.sh
source "$(dirname "$0")/_lib.sh"

read_payload
require_jq

path="$(extract_path)"
[[ -z "$path" || ! -f "$path" ]] && exit 0

file "$path" 2>/dev/null | grep -qiE 'text|json|xml|html|empty' || exit 0

case "$path" in
*.po | *.pot | *.svg | *.html.j2 | *.j2 | *.jinja | *.jinja2 | *.hbs | *.erb | *.liquid) exit 0 ;;
*/locales/* | */messages/* | */i18n/* | *.snap | */fixtures/* | */__snapshots__/* | */testdata/*) exit 0 ;;
esac

LC_ALL=C sed -i '' \
  -e 's/\xE2\x80\x94/-/g' \
  -e 's/\xE2\x80\x93/-/g' \
  -e 's/\xE2\x80\x9C/"/g' \
  -e 's/\xE2\x80\x9D/"/g' \
  -e 's/\xE2\x80\x98/'\''/g' \
  -e 's/\xE2\x80\x99/'\''/g' \
  -e 's/\xE2\x80\xA6/.../g' \
  -e 's/\xE2\x86\x92/->/g' \
  -e 's/\xE2\x86\x90/<-/g' \
  -e 's/\xE2\x87\x92/=>/g' \
  -e 's/\xE2\x80\xAA//g' \
  -e 's/\xE2\x80\xAB//g' \
  -e 's/\xE2\x80\xAC//g' \
  -e 's/\xE2\x80\xAD//g' \
  -e 's/\xE2\x80\xAE//g' \
  -e 's/\xE2\x81\xA6//g' \
  -e 's/\xE2\x81\xA7//g' \
  -e 's/\xE2\x81\xA8//g' \
  -e 's/\xE2\x81\xA9//g' \
  "$path" || true

exit 0
