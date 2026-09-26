#!/usr/bin/env bats
# Tests for hooks/format-dispatch.sh: each file is formatted by the one
# formatter its project opts into, per kit.yml's formatters table.
#
# Formatter binaries are recording stubs: each appends "<name> <args>" to
# $CALLS and changes nothing. The stub prettier answers --find-config-path
# with $FAKE_PRETTIER_CONFIG, so tests choose where "its" config lives.

setup() {
  HOOK="$BATS_TEST_DIRNAME/../hooks/format-dispatch.sh"
  export HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$HOME/.claude"
  unset CLAUDE_PLUGIN_ROOT CLAUDE_CONFIG_DIR FAKE_PRETTIER_CONFIG
  STUBS="$BATS_TEST_TMPDIR/stubs"
  CALLS="$BATS_TEST_TMPDIR/calls"
  mkdir -p "$STUBS"
  local name
  for name in biome dprint ruff black shfmt stylua gofmt taplo; do
    stub "$STUBS/$name" "$name"
  done
  cat >"$STUBS/prettier" <<EOF
#!/usr/bin/env bash
if [[ "\$1" == --find-config-path ]]; then
  [[ -n "\${FAKE_PRETTIER_CONFIG:-}" ]] || exit 1
  echo "\$FAKE_PRETTIER_CONFIG"
  exit 0
fi
echo "prettier \$*" >>"$CALLS"
EOF
  chmod +x "$STUBS/prettier"
  # Stubs first, then a bash 4+ ahead of macOS /bin/bash, then jq and yq.
  PATH="$STUBS:$(dirname "$(command -v bash)"):$(dirname "$(command -v jq)"):$(dirname "$(command -v yq)"):/usr/bin:/bin"
  REPO="$BATS_TEST_TMPDIR/repo"
  git init -q -b main "$REPO"
  REPO="$(cd -P "$REPO" && pwd)"
}

# stub <path> <label>: an executable that records "<label> <args>".
stub() {
  printf '#!/usr/bin/env bash\necho "%s $*" >>"%s"\n' "$2" "$CALLS" >"$1"
  chmod +x "$1"
}

format() {
  printf 'content\n' >"$1"
  jq -cn --arg p "$1" '{tool_input: {file_path: $p}}' | "$HOOK"
}

calls() {
  cat "$CALLS" 2>/dev/null || true
}

@test "no formatter signal leaves the file untouched" {
  run format "$REPO/README.md"
  [ "$status" -eq 0 ]
  [ -z "$(calls)" ]
  [ "$(cat "$REPO/README.md")" = "content" ]
}

@test "a Biome-only repo runs Biome, not Prettier" {
  printf '{}\n' >"$REPO/biome.json"
  mkdir -p "$REPO/src"
  run format "$REPO/src/app.ts"
  [ "$status" -eq 0 ]
  [ "$(calls)" = "biome format --write $REPO/src/app.ts" ]
}

@test "Biome and Prettier both configured: untouched, with a notice" {
  printf '{}\n' >"$REPO/biome.json"
  touch "$REPO/.prettierrc"
  export FAKE_PRETTIER_CONFIG="$REPO/.prettierrc"
  run format "$REPO/app.ts"
  [ "$status" -eq 0 ]
  [ -z "$(calls)" ]
  [[ "$output" == *"biome prettier are all configured"* ]]
}

@test "a Prettier config in the project formats with Prettier" {
  touch "$REPO/.prettierrc"
  export FAKE_PRETTIER_CONFIG=".prettierrc"
  run format "$REPO/notes.md"
  [ "$(calls)" = "prettier --write $REPO/notes.md" ]
}

@test "only a ~/.prettierrc leaves the file untouched" {
  touch "$HOME/.prettierrc"
  export FAKE_PRETTIER_CONFIG="$HOME/.prettierrc"
  run format "$REPO/notes.md"
  [ -z "$(calls)" ]
}

@test "a project-local binary wins over the one on PATH" {
  printf '{}\n' >"$REPO/biome.json"
  mkdir -p "$REPO/node_modules/.bin"
  stub "$REPO/node_modules/.bin/biome" local-biome
  run format "$REPO/app.ts"
  [ "$(calls)" = "local-biome format --write $REPO/app.ts" ]
}

@test "a bare [tool.ruff] table picks Ruff over Black" {
  printf '[tool.ruff]\n' >"$REPO/pyproject.toml"
  run format "$REPO/app.py"
  [ "$(calls)" = "ruff format $REPO/app.py" ]
}

@test "shfmt runs with no indent flag, so .editorconfig decides" {
  printf 'root = true\n' >"$REPO/.editorconfig"
  run format "$REPO/run.sh"
  [ "$(calls)" = "shfmt -w $REPO/run.sh" ]
}

@test "a signal above the project root is ignored" {
  printf 'root = true\n' >"$BATS_TEST_TMPDIR/.editorconfig"
  run format "$REPO/run.sh"
  [ -z "$(calls)" ]
}

@test "a path with spaces stays one argument" {
  printf '{}\n' >"$REPO/biome.json"
  mkdir -p "$REPO/my dir"
  run format "$REPO/my dir/app.ts"
  [ "$(calls)" = "biome format --write $REPO/my dir/app.ts" ]
}

@test "disabled_formatters in the overlay turns one off" {
  printf '{}\n' >"$REPO/biome.json"
  printf 'disabled_formatters: [biome]\n' >"$HOME/.claude/claude-kit.local.yml"
  run format "$REPO/app.ts"
  [ -z "$(calls)" ]
}

@test "an overlay-only formatter for .toml runs" {
  cat >"$HOME/.claude/claude-kit.local.yml" <<'YAML'
formatters:
  - name: taplo
    ext: [toml]
    signal_files: [taplo.toml]
    bin: taplo
    cmd: "{bin} format {file}"
YAML
  touch "$REPO/taplo.toml"
  run format "$REPO/Cargo.toml"
  [ "$(calls)" = "taplo format $REPO/Cargo.toml" ]
}

@test "a configured formatter that isn't installed leaves the file alone" {
  printf '{}\n' >"$REPO/biome.json"
  rm "$STUBS/biome"
  run format "$REPO/app.ts"
  [ "$status" -eq 0 ]
  [[ "$output" == *"biome is configured here but not installed"* ]]
}
