#!/usr/bin/env bash
# The shared repo, plus the sum fix left uncommitted.
set -euo pipefail
bash "$(dirname "$0")/../_fixture/repo.sh"
sed -i.bak 's/i = 2/i = 1/' calc.sh
rm calc.sh.bak
