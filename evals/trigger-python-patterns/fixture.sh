#!/usr/bin/env bash
# A one-module Python project, committed.
set -euo pipefail

cat >pyproject.toml <<'EOF'
[project]
name = "stats"
version = "0.1.0"
requires-python = ">=3.12"
EOF

cat >stats.py <<'EOF'
def mean(values: list[float]) -> float:
    if not values:
        raise ValueError("mean of an empty list")
    return sum(values) / len(values)
EOF

git init -q
git add .
git -c user.name=eval -c user.email=eval@example.com commit -qm "add stats"
