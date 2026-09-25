#!/usr/bin/env bash
# Idempotent symlink installer for the Claude config layout.
# Maps ~/.dotfiles/claude-kit/{hooks,skills,agents,bin,_stacks.yml} and the
# personal ~/.dotfiles/claude/{settings.json,CLAUDE.md,rules} onto
# ~/.claude/<same>. Dotfiles-specific: other adopters install claude-kit as a
# plugin instead (see claude-kit/README.md). Refuses to clobber non-symlinks unless --force, and
# never auto-removes a non-symlink directory (would destroy user data).
# Invoke with --non-interactive from install.sh; bare runs prompt on conflict.

set -euo pipefail

# pwd -P resolves the physical path so SOURCE_ROOT points at the real dotfiles
# checkout even when invoked through the symlinked ~/.claude/bin. A logical pwd
# here makes SOURCE_ROOT == ~/.claude, and the bin entry then self-symlinks (ELOOP).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
SOURCE_ROOT="$(cd "$SCRIPT_DIR/.." && pwd -P)"
PERSONAL_ROOT="$(cd "$SOURCE_ROOT/../claude" && pwd -P)"
TARGET_ROOT="$HOME/.claude"

root_for() {
  case "$1" in
  settings.json | CLAUDE.md | rules) echo "$PERSONAL_ROOT" ;;
  *) echo "$SOURCE_ROOT" ;;
  esac
}

INTERACTIVE=1
FORCE=0
for arg in "$@"; do
  case "$arg" in
  --non-interactive) INTERACTIVE=0 ;;
  --force) FORCE=1 ;;
  -h | --help)
    cat <<EOF
Usage: bootstrap.sh [--non-interactive] [--force]
  --non-interactive  Do not prompt; error on any conflict. For install.sh.
  --force            Overwrite non-symlink files without prompting.
                     Never removes non-symlink directories.
EOF
    exit 0
    ;;
  *)
    echo "Unknown argument: $arg" >&2
    exit 2
    ;;
  esac
done

ENTRIES=(settings.json CLAUDE.md hooks skills agents rules bin _stacks.yml)

mkdir -p "$TARGET_ROOT"

exit_code=0

for entry in "${ENTRIES[@]}"; do
  src="$(root_for "$entry")/$entry"
  dst="$TARGET_ROOT/$entry"

  if [[ ! -e "$src" && ! -L "$src" ]]; then
    echo "ERROR    source missing: $src" >&2
    exit_code=1
    continue
  fi

  if [[ -L "$dst" ]]; then
    actual="$(readlink "$dst")"
    if [[ "${actual%/}" == "$src" ]]; then
      echo "ok       $dst"
      continue
    fi
    echo "wrong    $dst -> $actual (expected $src)"
    if ((FORCE == 1)); then
      ln -sfn "$src" "$dst"
      echo "fixed    $dst"
    elif ((INTERACTIVE == 1)); then
      read -r -p "Replace symlink? [y/N] " ans
      if [[ "$ans" =~ ^[Yy]$ ]]; then
        ln -sfn "$src" "$dst"
        echo "fixed    $dst"
      else
        echo "skipped  $dst"
        exit_code=1
      fi
    else
      echo "ERROR    refusing to replace symlink in non-interactive mode: $dst" >&2
      exit_code=1
    fi
  elif [[ -e "$dst" ]]; then
    kind="file"
    [[ -d "$dst" ]] && kind="directory"
    echo "conflict $dst (regular $kind, expected symlink)"
    if [[ "$kind" == "directory" ]]; then
      # Never auto-remove a directory; it may hold user data not in dotfiles.
      echo "ERROR    not removing non-symlink directory; resolve manually: $dst" >&2
      exit_code=1
    elif ((FORCE == 1)); then
      rm -f "$dst"
      ln -s "$src" "$dst"
      echo "fixed    $dst"
    elif ((INTERACTIVE == 1)); then
      read -r -p "Remove file and create symlink? [y/N] " ans
      if [[ "$ans" =~ ^[Yy]$ ]]; then
        rm -f "$dst"
        ln -s "$src" "$dst"
        echo "fixed    $dst"
      else
        echo "skipped  $dst"
        exit_code=1
      fi
    else
      echo "ERROR    refusing to overwrite regular file in non-interactive mode: $dst" >&2
      exit_code=1
    fi
  else
    ln -s "$src" "$dst"
    echo "created  $dst"
  fi
done

setup_mcps() {
  if ! command -v claude >/dev/null 2>&1; then
    echo "skipped  claude CLI not found; re-run bootstrap.sh after installing Claude Code"
    return
  fi

  if ! claude mcp get context7 >/dev/null 2>&1; then
    claude mcp add -s user context7 -- npx -y @upstash/context7-mcp
    echo "created  mcp:context7"
  else
    echo "ok       mcp:context7"
  fi

  if ! claude mcp get foxhole >/dev/null 2>&1; then
    claude mcp add -s user foxhole -- foxhole mcp
    echo "created  mcp:foxhole"
  else
    echo "ok       mcp:foxhole"
  fi

  if ! claude mcp get playwright >/dev/null 2>&1; then
    claude mcp add -s user playwright -- npx @playwright/mcp@latest
    echo "created  mcp:playwright"
  else
    echo "ok       mcp:playwright"
  fi
}

setup_mcps

exit "$exit_code"
