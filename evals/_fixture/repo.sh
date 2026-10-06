#!/usr/bin/env bash
# A tiny bash project with one bug (sum skips its first argument), committed,
# for the cases that need a repo. Bash, not Go: the eval sandbox can't read a
# toolchain installed under the home directory. Runs in the case's empty
# workspace.
set -euo pipefail

cat >calc.sh <<'EOF'
#!/usr/bin/env bash
# sum prints the sum of its arguments.
sum() {
  local total=0 i
  for ((i = 2; i <= $#; i++)); do
    total=$((total + ${!i}))
  done
  echo "$total"
}
EOF

cat >test.sh <<'EOF'
#!/usr/bin/env bash
source "$(dirname "$0")/calc.sh"
got=$(sum 1 2 3)
if [[ $got != 6 ]]; then
  echo "FAIL: sum 1 2 3 = $got, want 6"
  exit 1
fi
echo ok
EOF
chmod +x calc.sh test.sh

git init -q
git add .
git -c user.name=eval -c user.email=eval@example.com commit -qm "add calc"
