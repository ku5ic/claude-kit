#!/usr/bin/env bash
# Lists the files that import a given file: the blast radius of editing it
# (rules/change.md section 3).
#
#   blast-radius.sh <file> [symbol]
#
# JS/TS: import, export-from, require() and import() specifiers. Relative
# specifiers resolve against the importing file; "@/" and "~/" aliases match
# on the path suffix (alias-match); a workspace package's name, or
# name/subpath, counts for every file in that package (workspace-match).
# Python: "import m" and "from m import x", absolute or relative. The module
# path follows the __init__.py chain, else it is root-relative minus src/.
# With [symbol], only consumers using it as a whole word are kept.
#
# Prints counts, then up to 50 "path:line <source|test> [label]" lines; test
# is decided by the test-patterns globs in kit.yml. Scans the repo's tracked
# and untracked-but-not-ignored files. Static only: a non-literal import() or
# require() prints "unresolvable imports present".
#
# Implemented by `kit blast-radius` (go/internal/blast).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=kit
source "$dir/kit" blast-radius "$@"
