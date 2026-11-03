#!/usr/bin/env bash
# The shared repo with the sum fix committed, so the comment describes
# working code instead of the bug.
set -euo pipefail
bash "$(dirname "$0")/../_fixture/repo.sh"
sed -i.bak 's/i = 2/i = 1/' calc.sh
rm calc.sh.bak
git -c user.name=eval -c user.email=eval@example.com commit -qam "fix sum skipping its first argument"
