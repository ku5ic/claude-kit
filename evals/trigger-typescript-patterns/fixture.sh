#!/usr/bin/env bash
# A one-module TypeScript project, committed.
set -euo pipefail

cat >package.json <<'EOF'
{ "name": "format", "version": "0.1.0", "type": "module", "devDependencies": { "typescript": "^5.9.0" } }
EOF

cat >tsconfig.json <<'EOF'
{ "compilerOptions": { "strict": true, "target": "ES2022", "module": "ESNext" } }
EOF

cat >format.ts <<'EOF'
export function formatPrice(cents: number): string {
  return `$${Math.floor(cents / 100)}.${String(cents % 100).padStart(2, "0")}`;
}
EOF

git init -q
git add .
git -c user.name=eval -c user.email=eval@example.com commit -qm "add format"
