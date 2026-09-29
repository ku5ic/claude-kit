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
#
# Implemented by `kit a11y-check` (go/internal/a11y).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=kit
source "$dir/kit" a11y-check "$@"
