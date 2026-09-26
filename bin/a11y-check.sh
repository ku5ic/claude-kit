#!/usr/bin/env bash
# Runs axe (@axe-core/cli) against a running page and prints a digest of its
# violations. Never installs anything and never starts a server.
#
#   a11y-check.sh <url>
#
# axe resolves from the nearest node_modules/.bin up to the project root,
# then PATH. The raw JSON (`axe --stdout`, an array of axe-core results) is
# saved next to where `scratch-dir.sh a11y-runtime <slug>` points, as .json.
# Digest, one line per violated rule:
#   <rule id>  <impact>  <wcag tags>  nodes=<n>  <first selector>
#
# Exit codes: 0 ran (violations or not), 1 axe failed, 2 usage,
# 3 axe not installed, 4 the URL does not answer.

set -euo pipefail

# shellcheck source=_lib.sh
source "$(dirname "$0")/_lib.sh"

if (($# != 1)); then
  echo "usage: a11y-check.sh <url>" >&2
  exit 2
fi
url="$1"
root="$(git rev-parse --show-toplevel 2>/dev/null || pwd -P)"

axe_bin="$(find_up "$(pwd -P)" "$root" node_modules/.bin/axe)"
[[ -n "$axe_bin" ]] || axe_bin="$(command -v axe || true)"
if [[ -z "$axe_bin" ]]; then
  echo "a11y-check: axe not found. Install it: npm install -D @axe-core/cli (or -g)"
  exit 3
fi

if ! curl -sS -o /dev/null --max-time 5 "$url" 2>/dev/null; then
  echo "a11y-check: $url does not answer."
  scripts=$'\n'"$(json_keys "$root/package.json" .scripts)"$'\n'
  for candidate in storybook dev; do
    if [[ "$scripts" == *$'\n'"$candidate"$'\n'* ]]; then
      pm="$(kit_nearest_pm_lockfile "$root" js)"
      pm="${pm%%:*}"
      echo "Start it with: ${pm:-npm} run $candidate"
      break
    fi
  done
  exit 4
fi

slug="$(printf '%s' "${url#*://}" | tr -c 'A-Za-z0-9' '-' | tr -s '-' | cut -c1-40)"
slug="${slug#-}"
report="$("$(dirname "$0")/scratch-dir.sh" a11y-runtime "${slug%-}")"
raw="${report%.md}.json"

if ! "$axe_bin" "$url" --stdout >"$raw"; then
  echo "a11y-check: axe failed on $url" >&2
  exit 1
fi

count="$(jq '[.[].violations[]] | length' "$raw")"
echo "a11y-check: $count violated rule(s) on $url (raw: $raw)"
jq -r '.[].violations[] | [
    .id,
    (.impact // "-"),
    ((.tags | map(select(startswith("wcag"))) | join(",")) | if . == "" then "-" else . end),
    "nodes=\(.nodes | length)",
    (.nodes[0].target // [] | map(if type == "array" then join(" ") else . end) | join(" >> "))
  ] | join("  ")' "$raw"
