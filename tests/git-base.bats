#!/usr/bin/env bats
# Characterization tests for bin/git-base.sh: pins which base it picks at
# each step of its detection order.

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../bin/git-base.sh"
  export HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$HOME"
  unset CLAUDE_CONFIG_DIR
  export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
}

# Bare origin with one commit on main, cloned to $BATS_TEST_TMPDIR/work.
# The clone sets origin/HEAD.
make_clone() {
  local seed="$BATS_TEST_TMPDIR/seed" origin="$BATS_TEST_TMPDIR/origin.git"
  git init -q -b main "$seed"
  git -C "$seed" commit -q --allow-empty -m init
  git clone -q --bare "$seed" "$origin"
  git clone -q "$origin" "$BATS_TEST_TMPDIR/work"
}

@test "an upstream other than this branch's own name wins" {
  make_clone
  cd "$BATS_TEST_TMPDIR/work"
  git switch -q -c feat --track origin/main

  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [ "$output" = "origin/main" ]
}

@test "with no upstream, origin/HEAD is used" {
  make_clone
  cd "$BATS_TEST_TMPDIR/work"
  git switch -q -c feat --no-track

  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [ "$output" = "origin/main" ]
}

@test "with no remote, a local main is used" {
  git init -q -b main "$BATS_TEST_TMPDIR/local"
  cd "$BATS_TEST_TMPDIR/local"
  git commit -q --allow-empty -m init
  git switch -q -c feat

  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [ "$output" = "main" ]
}

@test "an explicit ref that resolves wins over everything" {
  make_clone
  cd "$BATS_TEST_TMPDIR/work"
  git switch -q -c feat --track origin/main
  git branch -q other

  run "$SCRIPT" other
  [ "$status" -eq 0 ]
  [ "$output" = "other" ]
}

@test "nothing resolvable exits 1" {
  git init -q -b feat "$BATS_TEST_TMPDIR/empty"
  cd "$BATS_TEST_TMPDIR/empty"
  git commit -q --allow-empty -m init

  run "$SCRIPT"
  [ "$status" -eq 1 ]
  [ -z "$output" ]
}

# --diff and --log: the old git-diff-from-base.sh and git-log-from-base.sh.

# A local repo: main with one commit, feat with two more, one a merge-free
# change to a.txt.
make_branch() {
  git init -q -b main "$BATS_TEST_TMPDIR/local"
  cd "$BATS_TEST_TMPDIR/local"
  git commit -q --allow-empty -m init
  git switch -q -c feat
  echo one >a.txt
  git add a.txt
  git commit -q -m "add a"
  git commit -q --allow-empty -m "empty"
}

@test "--log lists this branch's commits against the base" {
  make_branch
  run "$SCRIPT" --log
  [ "$status" -eq 0 ]
  [ "$(printf '%s\n' "$output" | cut -d' ' -f2-)" = "empty
add a" ]
}

@test "--log passes flags through to git log" {
  make_branch
  run "$SCRIPT" --log -1
  [ "$(printf '%s\n' "$output" | cut -d' ' -f2-)" = "empty" ]
}

@test "--diff prints the three-dot diff against the base" {
  make_branch
  run "$SCRIPT" --diff --stat
  [ "$status" -eq 0 ]
  [[ "$output" == *"a.txt | 1 +"* ]]
}

@test "--log -n 5 takes 5 as the flag's value, not the base" {
  make_branch
  run "$SCRIPT" --log -n 1
  [ "$status" -eq 0 ]
  [ "$(printf '%s\n' "$output" | cut -d' ' -f2-)" = "empty" ]
}

@test "--diff base -- path limits the diff to the path, after the range" {
  make_branch
  echo two >b.txt
  git add b.txt
  git commit -q -m "add b"
  run "$SCRIPT" --diff main --stat -- a.txt
  [ "$status" -eq 0 ]
  [[ "$output" == *"a.txt"* ]]
  [[ "$output" != *"b.txt"* ]]
}

@test "a base that doesn't resolve exits 1 in every mode" {
  make_branch
  local mode
  for mode in "" --diff --log; do
    run "$SCRIPT" $mode notaref
    [ "$status" -eq 1 ] || {
      echo "mode '$mode' exited $status"
      return 1
    }
    [[ "$output" == *"'notaref' is not a ref"* ]]
  done
}

@test "a branch named log is still usable as the base" {
  make_branch
  git branch -q log main
  run "$SCRIPT" --log log
  [ "$(printf '%s\n' "$output" | wc -l | tr -d ' ')" = "2" ]
  run "$SCRIPT" log
  [ "$output" = "log" ]
}
