#!/usr/bin/env bash
# The shared repo with a one-line README to rewrite: a new file at the root
# would stop at guard-edit's root-file prompt, which nobody answers here.
set -euo pipefail
bash "$(dirname "$0")/../_fixture/repo.sh"
printf '# calc\n' >README.md
git add README.md
git -c user.name=eval -c user.email=eval@example.com commit -qm "add README stub"
