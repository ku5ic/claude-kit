#!/usr/bin/env bats
# Tests for ~/.dotfiles/claude-kit/hooks/plan-mode-context.sh.
#
# Run with: bats tests/

setup() {
  HOOK="$BATS_TEST_DIRNAME/../hooks/plan-mode-context.sh"
}

@test "plan mode prints the explore-patterns pointer" {
  run bash -c "printf '%s' '{\"permission_mode\":\"plan\"}' | '$HOOK'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"explore-patterns"* ]]
}

@test "default mode prints nothing" {
  run bash -c "printf '%s' '{\"permission_mode\":\"default\"}' | '$HOOK'"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "missing permission_mode prints nothing" {
  run bash -c "printf '%s' '{}' | '$HOOK'"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "invalid JSON exits clean with no output" {
  run bash -c "printf '%s' 'not json' | '$HOOK'"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}
