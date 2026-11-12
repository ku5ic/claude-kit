#!/usr/bin/env bash
# The shared repo, plus untracked files: two logs and a notes file someone
# might still want.
set -euo pipefail
bash "$(dirname "$0")/../_fixture/repo.sh"
mkdir logs
echo "run 1: ok" >logs/run1.log
echo "run 2: FAIL" >logs/run2.log
printf '%s\n' "TODO: sum should reject non-numbers" "ask about octal input" >notes.md
