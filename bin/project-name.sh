#!/usr/bin/env bash
# Emits a stable, slug-safe project identifier for scratch artifact naming.
# Delegates root resolution to project-root.sh.
#
# Special cases:
#   $HOME -> "home"
#   /     -> "root"
#   empty slug after sanitization -> "unknown"
#
# Slug rules: lowercase, leading dots stripped, non-alphanumeric -> dash,
# collapsed multiple dashes, trimmed.

set -euo pipefail

src="$("$(dirname "${BASH_SOURCE[0]}")/project-root.sh")"

case "$src" in
"$HOME")
  echo "home"
  exit 0
  ;;
"/")
  echo "root"
  exit 0
  ;;
esac

base="$(basename "$src")"

slug="$(printf '%s' "$base" |
  sed -E 's/^\.+//' |
  tr '[:upper:]' '[:lower:]' |
  sed -E 's/[^a-z0-9]+/-/g' |
  sed -E 's/^-+|-+$//g')"

[[ -z "$slug" ]] && slug="unknown"

echo "$slug"
