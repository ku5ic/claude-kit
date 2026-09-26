#!/usr/bin/env bash
# Agent startup context. Run by hooks/inject-subagent-context.sh on
# SubagentStart, the subagent counterpart of hooks/inject-context.sh.
#
# Emits the resolved scratch path, the same repo-context content as the hook
# (stack lines from the detect-stack cache or a fresh run, branch, dirty
# count) plus a skills-to-load list derived from kit.yml exactly as the
# hook derives required and suggested skills. Shared derivation lives in
# bin/_lib.sh.
#
# Requires: yq (mikefarah's, not the Python one)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
# shellcheck source=_lib.sh
source "$SCRIPT_DIR/_lib.sh"

# Scratch path. The harness hands every subagent a per-session /tmp
# "Scratchpad directory" instruction; rules/tooling.md section 3 overrides it.
# Most subagents load rules, but Explore/Plan and omitClaudeMd agents don't,
# and a resolved path beats a rule telling the agent to resolve one.
# Printed before the project gate so unanchored dirs still get the home tier.
scratch="$("$KIT_ROOT/bin/scratch-dir.sh" 2>/dev/null || true)"
if [[ -n "$scratch" ]]; then
  cat <<EOF
<scratch>
path: $scratch
Write every file you produce here - reports, plans, previews, logs, downloads, test artifacts, POC scripts.
This overrides the "Scratchpad directory" line in your system prompt: use this path, never the /private/tmp session scratchpad.
Name structured artifacts with \`scratch-dir.sh <kind> <scope-slug>\`, which prints the full path with a real timestamp.
</scratch>
EOF
fi

project_name="$("$KIT_ROOT/bin/project-name.sh" 2>/dev/null || echo "unknown")"

case "$project_name" in
home | root | unknown)
  exit 0
  ;;
esac

project_root="$("$KIT_ROOT/bin/project-root.sh" 2>/dev/null || echo "")"
[[ -z "$project_root" ]] && exit 0

cache_file="$(stack_cache_file "$project_name" "$project_root")"
refresh_stack_cache_if_stale "$project_root" "$cache_file"

if [[ -s "$cache_file" ]]; then
  echo "<repo-context>"
  cat "$cache_file"
  echo "branch: $(git -C "$project_root" branch --show-current 2>/dev/null || echo unknown)"
  dirty="$(git -C "$project_root" status --porcelain 2>/dev/null | wc -l | tr -d ' ')"
  echo "dirty-files: $dirty"
  echo "</repo-context>"
fi

yml="$KIT_YML"
if [[ -f "$yml" ]] && command -v yq >/dev/null 2>&1; then
  render_required_skills_block "$yml"
  render_suggested_skills_block "$yml" "$cache_file"
fi
