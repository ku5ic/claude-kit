#!/usr/bin/env bash
# Links claude-kit's always-on rules into ~/.claude/rules/claude-kit. Plugins
# cannot ship rules, so this is the one manual step after installing the
# plugin. Run it from a stable checkout (the marketplace clone under
# ~/.claude/plugins/marketplaces/, or your own clone), never from the
# versioned plugin cache, whose path changes on every update.

set -euo pipefail

kit_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
case "$kit_dir" in
*/.claude/plugins/cache/*)
  echo "install-rules.sh: run this from the marketplace clone or your own checkout, not the plugin cache ($kit_dir)" >&2
  exit 1
  ;;
esac

dst="$HOME/.claude/rules/claude-kit"
mkdir -p "$(dirname "$dst")"

if [[ -e "$dst" && ! -L "$dst" ]]; then
  echo "install-rules.sh: $dst exists and is not a symlink; move it aside first" >&2
  exit 1
fi

ln -sfn "$kit_dir/rules" "$dst"
echo "linked $dst -> $kit_dir/rules"
