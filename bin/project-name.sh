#!/usr/bin/env bash
# Emits a stable, slug-safe project identifier for scratch artifact naming,
# from project-root.sh's root.
#
# Special cases:
#   $HOME -> "home"
#   /     -> "root"
#   empty slug after sanitization -> "unknown"
#
# Slug rules: lowercase, leading dots stripped, non-alphanumeric -> dash,
# collapsed multiple dashes, trimmed.
#
# Implemented by `kit project-name` (go/internal/project).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=kit
source "$dir/kit" project-name "$@"
