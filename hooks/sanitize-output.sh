#!/usr/bin/env bash
# PostToolUse hook for Write, Edit, MultiEdit: always strips bidi control
# characters (Trojan Source) from written files. With
# CLAUDE_SANITIZE_TYPOGRAPHY=1 it also rewrites em dashes, smart quotes, etc.
# to ASCII.
HOOK_NAME="sanitize-output.sh"
# shellcheck source=../bin/_lib.sh
source "$(dirname "$0")/../bin/_lib.sh"
kit_hook_init

read_payload
require_jq

path="$(extract_path)"
[[ -z "$path" || ! -f "$path" ]] && exit 0

file "$path" 2>/dev/null | grep -qiE 'text|json|xml|html|empty' || exit 0

case "$path" in
*.po | *.pot | *.svg | *.html.j2 | *.j2 | *.jinja | *.jinja2 | *.hbs | *.erb | *.liquid) exit 0 ;;
*/locales/* | */messages/* | */i18n/* | *.snap | */fixtures/* | */__snapshots__/* | */testdata/*) exit 0 ;;
esac

# -i<suffix> is the only in-place form BSD and GNU sed both accept; the
# PID suffix avoids clobbering a real "$path.bak".
sed_in_place() {
  LC_ALL=C sed -i".sanitize-$$" "$@" "$path" || true
  rm -f "$path.sanitize-$$"
}

sed_in_place \
  -e 's/\xE2\x80\xAA//g' \
  -e 's/\xE2\x80\xAB//g' \
  -e 's/\xE2\x80\xAC//g' \
  -e 's/\xE2\x80\xAD//g' \
  -e 's/\xE2\x80\xAE//g' \
  -e 's/\xE2\x81\xA6//g' \
  -e 's/\xE2\x81\xA7//g' \
  -e 's/\xE2\x81\xA8//g' \
  -e 's/\xE2\x81\xA9//g'

if [[ "${CLAUDE_SANITIZE_TYPOGRAPHY:-0}" == "1" ]]; then
  sed_in_place \
    -e 's/\xE2\x80\x94/-/g' \
    -e 's/\xE2\x80\x93/-/g' \
    -e 's/\xE2\x80\x9C/"/g' \
    -e 's/\xE2\x80\x9D/"/g' \
    -e 's/\xE2\x80\x98/'\''/g' \
    -e 's/\xE2\x80\x99/'\''/g' \
    -e 's/\xE2\x80\xA6/.../g' \
    -e 's/\xE2\x86\x92/->/g' \
    -e 's/\xE2\x86\x90/<-/g' \
    -e 's/\xE2\x87\x92/=>/g'
fi

exit 0
