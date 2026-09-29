#!/usr/bin/env bash
# PostToolUse hook for Edit, Write, MultiEdit: formats the edited file with
# the project's own formatter and config, per kit.yml's formatters table.
# A file no formatter claims, or one two formatters claim (a migration in
# progress), is left byte-identical. Never installs anything: binaries come
# from the project's node_modules/.bin or virtualenv, else PATH. Shell files
# also get a non-blocking shellcheck pass on stderr.
#
# Implemented by `kit hook format-dispatch` (go/internal/hooks).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=../bin/kit
source "$dir/../bin/kit" hook format-dispatch "$@"
