#!/usr/bin/env bats
# Characterization tests for bin/detect-stack.sh: pins today's report on
# three fixtures so later refactors change it only on purpose.
#
# Runs the real script against the real kit.yml; HOME is faked so the
# stack-list cache lands in the test's tmpdir.

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../bin/detect-stack.sh"
  export HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$HOME"
  unset CLAUDE_PLUGIN_ROOT CLAUDE_CONFIG_DIR
}

# Creates a git repo at $1 and prints its physical path, which is what
# git rev-parse (and so the report's root: line) uses.
make_repo() {
  mkdir -p "$1"
  git init -q -b main "$1"
  (cd -P "$1" && pwd)
}

@test "pnpm Next.js repo reports js with extras and the pnpm tag" {
  local root
  root="$(make_repo "$BATS_TEST_TMPDIR/next")"
  printf '{"dependencies":{"next":"15.0.0","react":"19.0.0"},"devDependencies":{"typescript":"5.6.0"}}\n' >"$root/package.json"
  printf '{}\n' >"$root/tsconfig.json"
  touch "$root/pnpm-lock.yaml"

  cd "$root"
  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [ "$output" = "root: $root
js: yes (typescript,next,react) [pnpm]
versions: react 19.0.0 (declared), next 15.0.0 (declared), typescript 5.6.0 (declared)" ]
}

@test "uv Django repo reports python with django and the uv tag" {
  local root
  root="$(make_repo "$BATS_TEST_TMPDIR/django")"
  printf '[project]\nname = "x"\ndependencies = ["django>=5.0"]\n' >"$root/pyproject.toml"
  touch "$root/uv.lock" "$root/manage.py"

  cd "$root"
  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [ "$output" = "root: $root
python: yes (django) [uv]" ]
}

@test "repo with no sentinel prints nothing" {
  local root
  root="$(make_repo "$BATS_TEST_TMPDIR/none")"

  cd "$root"
  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "every subproject is detected, each stack with its own ecosystem's manager" {
  local root
  root="$(make_repo "$BATS_TEST_TMPDIR/mono")"
  printf '{"name":"root","private":true}\n' >"$root/package.json"
  printf 'packages:\n  - "packages/*"\n' >"$root/pnpm-workspace.yaml"
  touch "$root/pnpm-lock.yaml"
  mkdir -p "$root/packages/a" "$root/services/api"
  printf '{"name":"a","dependencies":{"react":"19.0.0"}}\n' >"$root/packages/a/package.json"
  printf '[project]\nname = "api"\n' >"$root/services/api/pyproject.toml"
  touch "$root/services/api/uv.lock"
  git -C "$root" add -A

  cd "$root"
  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [ "$output" = "root: $root
js: yes (react) [pnpm] at ., packages/a
python: yes [uv] at services/api
monorepo: yes (pnpm-workspaces)
versions [packages/a]: react 19.0.0 (declared)" ]
}

@test "versions: installed per subproject, each on its own line" {
  local root app
  root="$(make_repo "$BATS_TEST_TMPDIR/apps")"
  printf '{"name":"root","private":true}\n' >"$root/package.json"
  printf 'packages:\n  - "apps/*"\n' >"$root/pnpm-workspace.yaml"
  for app in admin:18.3.1 web:19.1.0; do
    mkdir -p "$root/apps/${app%%:*}/node_modules/react"
    printf '{"name":"%s","dependencies":{"react":"^%s"}}\n' "${app%%:*}" "${app#*:}" >"$root/apps/${app%%:*}/package.json"
    printf '{"name":"react","version":"%s"}\n' "${app#*:}" >"$root/apps/${app%%:*}/node_modules/react/package.json"
  done

  cd "$root"
  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"
versions [apps/admin]: react 18.3.1 (installed)
versions [apps/web]: react 19.1.0 (installed)"* ]]
}

@test "versions: a workspace-root uv.lock, else a requirements.txt pin" {
  local root
  root="$(make_repo "$BATS_TEST_TMPDIR/py")"
  printf '[project]\nname = "x"\n' >"$root/pyproject.toml"
  printf '[[package]]\nname = "django"\nversion = "5.1.2"\n\n[[package]]\nname = "pydantic"\nversion = "2.9.0"\n' >"$root/uv.lock"
  mkdir -p "$root/services/api"
  printf '[project]\nname = "api"\n' >"$root/services/api/pyproject.toml"
  printf 'FastAPI==0.115.0\n' >"$root/services/api/requirements.txt"
  git -C "$root" add -A

  cd "$root"
  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"versions: django 5.1.2 (locked), pydantic 2.9.0 (locked)
versions [services/api]: django 5.1.2 (locked), fastapi 0.115.0 (pinned), pydantic 2.9.0 (locked)"* ]]
}

@test "the stack-line parser still reads extras from the new format" {
  printf '%s\n' "root: /x" "js: yes (react) [pnpm] at ., packages/a" "python: yes [uv] at services/api" "versions [packages/a]: react 19.0.0 (declared)" >"$BATS_TEST_TMPDIR/cache"
  # shellcheck source=../bin/_lib.sh
  source "$BATS_TEST_DIRNAME/../bin/_lib.sh"
  run stacks_signals_from_cache "$BATS_TEST_TMPDIR/cache"
  [ "$output" = "js
js+react
python" ]
}

# The user overlay at ~/.claude/claude-kit.local.yml merges over kit.yml.

write_overlay() {
  mkdir -p "$HOME/.claude"
  cat >"$HOME/.claude/claude-kit.local.yml"
}

@test "a stack added by the overlay is detected" {
  local root
  root="$(make_repo "$BATS_TEST_TMPDIR/custom")"
  touch "$root/custom.marker"
  write_overlay <<'YAML'
stacks:
  custom:
    sentinels:
      - name: custom.marker
    skills: []
YAML

  cd "$root"
  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [ "$output" = "root: $root
custom: yes" ]
}

@test "the overlay's arrays append to kit.yml's instead of replacing them" {
  local root
  root="$(make_repo "$BATS_TEST_TMPDIR/next")"
  printf '{"dependencies":{"react":"19.0.0"}}\n' >"$root/package.json"
  touch "$root/extra.marker"
  write_overlay <<'YAML'
stacks:
  js:
    extras:
      - name: marked
        file: extra.marker
YAML

  cd "$root"
  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"js: yes (react,marked) [npm]"* ]]
}

@test "editing the overlay invalidates the merged copy" {
  local root
  root="$(make_repo "$BATS_TEST_TMPDIR/custom")"
  touch "$root/custom.marker"
  write_overlay <<'YAML'
stacks: {}
YAML
  cd "$root"
  run "$SCRIPT"
  [ -z "$output" ]

  write_overlay <<'YAML'
stacks:
  custom:
    sentinels:
      - name: custom.marker
    skills: []
YAML
  # A future mtime: the rewrite can land in the same second as the merge.
  touch -t 209901010000 "$HOME/.claude/claude-kit.local.yml"
  run "$SCRIPT"
  [[ "$output" == *"custom: yes"* ]]
}
