#!/usr/bin/env bats
# bin/blast-radius.sh: consumers of a file across relative, alias, workspace,
# and Python imports, split into source and test.

setup() {
  load helper
  kit_test_home
  export HOME="$FAKE_HOME"
  SCRIPT="$BATS_TEST_DIRNAME/../bin/blast-radius.sh"
}

# Creates a git repo at $1, cds into it.
make_repo() {
  mkdir -p "$1"
  git init -q -b main "$1"
  cd "$1"
}

# write <path> <content>
write() {
  mkdir -p "$(dirname "$1")"
  printf '%s\n' "$2" >"$1"
}

make_ts_fixture() {
  make_repo "$BATS_TEST_TMPDIR/ts"
  write src/lib/format.ts 'export const formatDate = () => "";'
  write src/app/page.ts "import { formatDate } from '../lib/format';"
  write src/components/Card.tsx 'import { formatDate } from "@/lib/format";'
  write src/lib/format.test.ts "import { formatDate } from './format.js';"
  write src/other.ts "import { formatMoney } from './lib/money';"
}

@test "TS: relative, alias, and test consumers" {
  make_ts_fixture
  run "$SCRIPT" src/lib/format.ts
  [ "$status" -eq 0 ]
  [ "$output" = "blast-radius: src/lib/format.ts
consumers: 3 (source 2, test 1)
src/app/page.ts:1 source
src/components/Card.tsx:1 source alias-match
src/lib/format.test.ts:1 test" ]
}

@test "a symbol keeps only consumers that use it" {
  make_ts_fixture
  write src/app/page.ts "import { other } from '../lib/format';"
  run "$SCRIPT" src/lib/format.ts formatDate
  [[ "$output" == *"consumers: 2 (source 1, test 1)"* ]]
  [[ "$output" != *"src/app/page.ts"* ]]
}

@test "an index file is reached through its directory" {
  make_repo "$BATS_TEST_TMPDIR/idx"
  write src/ui/index.ts 'export {};'
  write src/main.ts "import { Button } from './ui';"
  run "$SCRIPT" src/ui/index.ts
  [[ "$output" == *"src/main.ts:1 source"* ]]
}

@test "an index file is reached by a bare '..' or './' from below or beside it" {
  make_repo "$BATS_TEST_TMPDIR/idx2"
  write src/foo/index.ts 'export const a = 1;'
  write src/foo/sub/b.ts "import { a } from '..';"
  write src/foo/c.ts 'import { a } from "./";'
  run "$SCRIPT" src/foo/index.ts
  [[ "$output" == *"consumers: 2 (source 2, test 0)"* ]]
}

@test "a non-literal import() is flagged" {
  make_ts_fixture
  write src/lazy.ts 'const m = await import(path);'
  run "$SCRIPT" src/lib/format.ts
  [[ "$output" == *"unresolvable imports present"* ]]
}

@test "workspace: a package-name import is a workspace-match" {
  make_repo "$BATS_TEST_TMPDIR/ws"
  write package.json '{"name":"root","private":true}'
  write pnpm-workspace.yaml $'packages:\n  - apps/*\n  - packages/*'
  write packages/ui/package.json '{"name":"@acme/ui"}'
  write packages/ui/src/index.ts 'export {};'
  write apps/web/package.json '{"name":"web"}'
  write apps/web/src/page.tsx "import { Button } from '@acme/ui';"
  write apps/web/src/other.tsx "import { x } from '@acme/uikit';"
  run "$SCRIPT" packages/ui/src/index.ts
  [ "$status" -eq 0 ]
  [[ "$output" == *"consumers: 1 (source 1, test 0)"* ]]
  [[ "$output" == *"apps/web/src/page.tsx:1 source workspace-match"* ]]
}

@test "Python: absolute, from-parent, relative, and test imports" {
  make_repo "$BATS_TEST_TMPDIR/py"
  write app/__init__.py ''
  write app/models.py 'class User: pass'
  write app/views.py 'from .models import User'
  write app/admin.py 'from app import models'
  write scripts/seed.py 'import app.models as m'
  write tests/test_models.py 'from app.models import User'
  write app/unrelated.py 'from app import views'
  run "$SCRIPT" app/models.py
  [ "$status" -eq 0 ]
  [ "$output" = "blast-radius: app/models.py
consumers: 4 (source 3, test 1)
app/admin.py:1 source
app/views.py:1 source
scripts/seed.py:1 source
tests/test_models.py:1 test" ]
}

@test "an unsupported file type exits 2" {
  make_repo "$BATS_TEST_TMPDIR/other"
  write a.rb 'x'
  run "$SCRIPT" a.rb
  [ "$status" -eq 2 ]
}
