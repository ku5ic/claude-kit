#!/usr/bin/env bash
# PostToolUse hook for Edit, Write, MultiEdit: formats the edited file with
# the project's own formatter and config, per kit.yml's formatters table.
# A file no formatter claims, or one two formatters claim (a migration in
# progress), is left byte-identical. Never installs anything: binaries come
# from the project's node_modules/.bin or virtualenv, else PATH.
HOOK_NAME="format-dispatch.sh"
# shellcheck source=../bin/_lib.sh
source "$(dirname "$0")/../bin/_lib.sh"
kit_hook_init

read_payload
require_jq

path="$(extract_path)"
[[ -z "$path" || ! -f "$path" ]] && exit 0

case "$path" in
*/.claude/scratch/* | */scratch/*) exit 0 ;;
esac

[[ "${path##*/}" == *.* ]] || exit 0
ext="${path##*.}"
ext="${ext,,}"
dir="$(cd -P "${path%/*}" 2>/dev/null && pwd)" || exit 0
root="$(git -C "$dir" rev-parse --show-toplevel 2>/dev/null || printf '%s' "$dir")"
kit_stacks_load

# Nearest project-local copy of binary $1, else PATH; nothing when absent.
resolve_bin() {
  local sub found
  for sub in node_modules/.bin .venv/bin venv/bin; do
    found="$(find_up "$dir" "$root" "$sub/$1")"
    if [[ -n "$found" && -x "$found" ]]; then
      printf '%s\n' "$found"
      return 0
    fi
  done
  command -v "$1" 2>/dev/null || true
}

# True when formatter index $1 has a signal for this file; binary in $2.
has_signal() {
  local i="$1" bin="$2" toml_file cfg
  local -a names
  if [[ "${KIT_FMT_FILES[i]}" != - ]]; then
    read -ra names <<<"${KIT_FMT_FILES[i]}"
    [[ -n "$(find_up "$dir" "$root" "${names[@]}")" ]] && return 0
  fi
  if [[ "${KIT_FMT_TOMLS[i]}" != - ]]; then
    toml_file="$(find_up "$dir" "$root" "${KIT_FMT_TOMLS[i]%% *}")"
    [[ -n "$toml_file" ]] && toml_has "$toml_file" "${KIT_FMT_TOMLS[i]#* }" && return 0
  fi
  # Prettier's own lookup, which also finds a ~/.prettierrc: only a config
  # inside the project root counts, or every repo would get Prettier.
  if [[ "${KIT_FMT_PRETTIER[i]}" == true && -n "$bin" ]]; then
    cfg="$(cd "$dir" && "$bin" --find-config-path "$path" 2>/dev/null)" || cfg=""
    if [[ -n "$cfg" ]]; then
      [[ "$cfg" == /* ]] || cfg="$dir/$cfg"
      cfg="$(kit_physical_path "$cfg")"
      [[ "$cfg" == "$root"/* ]] && return 0
    fi
  fi
  return 1
}

hits=()
hit_bins=()
for ((i = 0; i < ${#KIT_FMT_NAMES[@]}; i++)); do
  [[ " ${KIT_DISABLED_FORMATTERS[*]:-} " == *" ${KIT_FMT_NAMES[i]} "* ]] && continue
  [[ " ${KIT_FMT_EXTS[i]} " == *" $ext "* ]] || continue
  bin="$(resolve_bin "${KIT_FMT_BINS[i]}")"
  has_signal "$i" "$bin" || continue
  hits+=("$i")
  hit_bins+=("$bin")
done

if ((${#hits[@]} > 1)); then
  names=()
  for i in "${hits[@]}"; do
    names+=("${KIT_FMT_NAMES[i]}")
  done
  echo "format-dispatch: left ${path##*/} unformatted; ${names[*]} are all configured for it here" >&2
elif ((${#hits[@]} == 1)); then
  i="${hits[0]}"
  bin="${hit_bins[0]}"
  if [[ -z "$bin" ]]; then
    echo "format-dispatch: ${KIT_FMT_NAMES[i]} is configured here but not installed; ${path##*/} left unformatted" >&2
  else
    # Substituted word by word, so a path with spaces stays one argument.
    parts=()
    read -ra words <<<"${KIT_FMT_CMDS[i]}"
    for word in "${words[@]}"; do
      word="${word//\{bin\}/$bin}"
      parts+=("${word//\{file\}/$path}")
    done
    (cd "$dir" && "${parts[@]}") >/dev/null || true
  fi
fi

if [[ "$ext" == sh || "$ext" == bash ]] && command -v shellcheck >/dev/null 2>&1; then
  shellcheck "$path" >&2 || true
fi

exit 0
