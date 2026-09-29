#!/usr/bin/env bash
# Agent startup context. Run by hooks/inject-subagent-context.sh on
# SubagentStart, the subagent counterpart of hooks/inject-context.sh.
#
# Emits the resolved scratch path, the same repo-context content as the hook
# (stack lines from the detect-stack cache or a fresh run, branch, dirty
# count) plus the required and suggested skills derived from kit.yml exactly
# as the hook derives them: both use go/internal/stackctx.
#
# Implemented by `kit agent-context` (go/internal/hooks).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=kit
source "$dir/kit" agent-context "$@"
