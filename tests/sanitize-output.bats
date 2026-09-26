#!/usr/bin/env bats
# Tests for ~/.dotfiles/claude-kit/hooks/sanitize-output.sh.
#
# Each test writes a fixture file containing a typographic em dash, feeds a
# synthetic Write/Edit payload (pointing at that file) to the hook, and
# checks whether the em dash survived: still present means the path's
# extension/directory excluded it from the rewrite.
#
# Run with: bats tests/

setup() {
  HOOK="$BATS_TEST_DIRNAME/../hooks/sanitize-output.sh"
  WORK="$BATS_TEST_TMPDIR/work"
  mkdir -p "$WORK"
}

# make_fixture <rel-path>  creates $WORK/<rel-path> containing a raw UTF-8
# em dash (U+2014), creating parent directories as needed.
make_fixture() {
  local rel="$1"
  mkdir -p "$WORK/$(dirname "$rel")"
  printf 'em dash \xe2\x80\x94 here\n' >"$WORK/$rel"
}

# run_hook <rel-path>  feeds the Write-payload shape to the hook, with the
# typography rewrite enabled so the em dash marker exercises the exclusions.
run_hook() {
  local rel="$1"
  jq -n --arg fp "$WORK/$rel" '{tool_input: {file_path: $fp}}' |
    CLAUDE_SANITIZE_TYPOGRAPHY=1 "$HOOK"
}

# run_hook_default <rel-path>  same payload, typography flag unset.
run_hook_default() {
  local rel="$1"
  jq -n --arg fp "$WORK/$rel" '{tool_input: {file_path: $fp}}' |
    env -u CLAUDE_SANITIZE_TYPOGRAPHY "$HOOK"
}

# make_bidi_fixture <rel-path>  creates a file holding an em dash and a
# right-to-left override (U+202E).
make_bidi_fixture() {
  local rel="$1"
  mkdir -p "$WORK/$(dirname "$rel")"
  printf 'em dash \xe2\x80\x94 rlo \xe2\x80\xae here\n' >"$WORK/$rel"
}

bidi_present() {
  grep -q $'\xe2\x80\xae' "$WORK/$1"
}

# em_dash_present <rel-path>  true (0) when the raw em dash bytes are still
# in the file, i.e. the hook skipped rewriting it.
em_dash_present() {
  grep -q $'\xe2\x80\x94' "$WORK/$1"
}

@test "baseline: a non-excluded path gets its em dash rewritten" {
  make_fixture "src/app.ts"
  run run_hook "src/app.ts"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
  ! em_dash_present "src/app.ts"
}

@test "default: typography is kept, bidi control characters are stripped" {
  make_bidi_fixture "src/app.ts"
  run run_hook_default "src/app.ts"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
  em_dash_present "src/app.ts"
  ! bidi_present "src/app.ts"
}

@test "typography flag on: bidi control characters are stripped too" {
  make_bidi_fixture "src/app.ts"
  run run_hook "src/app.ts"
  [ "$status" -eq 0 ]
  ! em_dash_present "src/app.ts"
  ! bidi_present "src/app.ts"
}

@test "typography flag on: quotes, ellipsis, and arrows become ASCII" {
  mkdir -p "$WORK/src"
  printf '\xe2\x80\x9cq\xe2\x80\x9d \xe2\x80\x98s\xe2\x80\x99 \xe2\x80\xa6 \xe2\x86\x92 \xe2\x86\x90 \xe2\x87\x92 \xe2\x80\x93\n' >"$WORK/src/q.md"
  run run_hook "src/q.md"
  [ "$status" -eq 0 ]
  [ "$(cat "$WORK/src/q.md")" = "\"q\" 's' ... -> <- => -" ]
}

@test "skip: */locales/* directory is not rewritten" {
  make_fixture "locales/en.json"
  run run_hook "locales/en.json"
  [ "$status" -eq 0 ]
  em_dash_present "locales/en.json"
}

@test "skip: */locales/* nested deeper in the tree is not rewritten" {
  make_fixture "a/locales/b/en.json"
  run run_hook "a/locales/b/en.json"
  [ "$status" -eq 0 ]
  em_dash_present "a/locales/b/en.json"
}

@test "skip: */messages/* directory is not rewritten" {
  make_fixture "messages/en.json"
  run run_hook "messages/en.json"
  [ "$status" -eq 0 ]
  em_dash_present "messages/en.json"
}

@test "skip: */i18n/* directory is not rewritten" {
  make_fixture "i18n/en.json"
  run run_hook "i18n/en.json"
  [ "$status" -eq 0 ]
  em_dash_present "i18n/en.json"
}

@test "skip: *.snap file is not rewritten" {
  make_fixture "foo.snap"
  run run_hook "foo.snap"
  [ "$status" -eq 0 ]
  em_dash_present "foo.snap"
}

@test "skip: */fixtures/* directory is not rewritten" {
  make_fixture "fixtures/data.json"
  run run_hook "fixtures/data.json"
  [ "$status" -eq 0 ]
  em_dash_present "fixtures/data.json"
}

@test "skip: */__snapshots__/* directory is not rewritten" {
  make_fixture "__snapshots__/x.snap"
  run run_hook "__snapshots__/x.snap"
  [ "$status" -eq 0 ]
  em_dash_present "__snapshots__/x.snap"
}

@test "skip: */testdata/* directory is not rewritten" {
  make_fixture "testdata/x.json"
  run run_hook "testdata/x.json"
  [ "$status" -eq 0 ]
  em_dash_present "testdata/x.json"
}

@test "no false positive: a directory that merely contains 'locales' as a substring is still rewritten" {
  make_fixture "mylocalesdir/en.json"
  run run_hook "mylocalesdir/en.json"
  [ "$status" -eq 0 ]
  ! em_dash_present "mylocalesdir/en.json"
}

@test "no false positive: a .snap.bak file does not match the *.snap extension pattern" {
  make_fixture "foo.snap.bak"
  run run_hook "foo.snap.bak"
  [ "$status" -eq 0 ]
  ! em_dash_present "foo.snap.bak"
}
