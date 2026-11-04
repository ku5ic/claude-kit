#!/bin/sh
# The slop scan for claude.ai chat, on stdin, from the unslop skill's folder.
# Usage: scan.sh [surface]
# Exits 0 when nothing blocks, 1 when a hit does, and anything else when the
# scan itself didn't run: the skill then falls back to reference/lexicon.md.
set -u

here=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd) || exit 3
tmp=$(mktemp -d) || exit 3
trap 'rm -rf "$tmp"' EXIT

# A skill's folder may not be executable, and kit needs a config dir.
cp "$here/scripts/kit-linux-amd64" "$tmp/kit" || exit 3
chmod +x "$tmp/kit" || exit 3

status=0
out=$(CLAUDE_PLUGIN_ROOT="$here" CLAUDE_CONFIG_DIR="$tmp" "$tmp/kit" slop --surface "${1:-chat}" -) || status=$?

# kit also exits 1 when it can't load its config, with nothing on stdout.
if [ "$status" -eq 1 ] && [ -z "$out" ]; then
  exit 3
fi
[ -z "$out" ] || printf '%s\n' "$out"
exit "$status"
