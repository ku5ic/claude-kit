#!/usr/bin/env bash
# Requires bash 4.0+. Personal dotfiles; bash 5.x is installed via Brewfile.
# Emits a compact stack report for the current project or a nearby ancestor.
# Output is terse on purpose. Each line is meant to be scanned by Claude in
# under a few hundred tokens of context.
#
# All stack knowledge (sentinels, search_dirs, extras detection rules, skills)
# lives in _stacks.yml at the kit root ($KIT_ROOT, see _lib.sh). To add a new stack or extend an existing one, edit
# that file only. No edits to this script are required.
#
# Requires: yq (mikefarah, installed via Brewfile)

set -euo pipefail

# shellcheck source=_lib.sh
source "$(dirname "$0")/_lib.sh"

STACKS_YML="$KIT_ROOT/_stacks.yml"
ROOT="$("$KIT_ROOT/bin/project-root.sh")"
cd "$ROOT"

# Load stack names in document order
mapfile -t STACK_NAMES < <(yq '.stacks | keys | .[]' "$STACKS_YML")

default_search_dirs=(. frontend client web app backend server api)

# Returns the directory under ROOT where the stack was found, or empty.
find_stack_dir() {
  local stack="$1"
  local dirs raw_dirs
  # mikefarah yq outputs "null" (exit 0) when a key is absent, not a non-zero
  # exit. Check for empty or literal "null" and fall back to defaults.
  raw_dirs="$(yq ".stacks.${stack}.search_dirs[]" "$STACKS_YML" 2>/dev/null || true)"
  if [[ -z "$raw_dirs" || "$raw_dirs" == "null" ]]; then
    dirs=("${default_search_dirs[@]}")
  else
    mapfile -t dirs <<<"$raw_dirs"
  fi
  local sentinels
  mapfile -t sentinels < <(yq ".stacks.${stack}.sentinels[].name" "$STACKS_YML")
  for dir in "${dirs[@]}"; do
    for sentinel in "${sentinels[@]}"; do
      if [[ -f "$dir/$sentinel" ]]; then
        echo "$dir"
        return 0
      fi
    done
  done
}

emit_stack() {
  local name="$1" loc="$2" parts="$3" suffix="$4"
  local line="$name: yes"
  [[ "$loc" != "." ]] && line+=" at $loc/"
  [[ -n "$parts" ]] && line+=" ($parts)"
  [[ -n "$suffix" ]] && line+=" $suffix"
  echo "$line"
}

# Evaluates a single extras entry detection rule against $loc.
# Prints the extra name if the rule matches; prints nothing otherwise.
# rule types: dep, file, grep+in, any_of
eval_extra() {
  local stack="$1" loc="$2" idx="$3"

  local name
  name="$(yq ".stacks.${stack}.extras[${idx}].name" "$STACKS_YML")"

  local rename
  rename="$(yq ".stacks.${stack}.extras[${idx}].rename // \"\"" "$STACKS_YML")"
  local token="${rename:-$name}"

  # dep: package.json dependency check
  local dep
  dep="$(yq ".stacks.${stack}.extras[${idx}].dep // \"\"" "$STACKS_YML")"
  if [[ -n "$dep" ]]; then
    if command -v jq >/dev/null 2>&1 && [[ -f "$loc/package.json" ]]; then
      local found
      found="$(jq -r '((.dependencies // {}) + (.devDependencies // {})) | keys[]' "$loc/package.json" 2>/dev/null | grep -Fx "$dep" || true)"
      if [[ -n "$found" ]]; then echo "$token"; fi
    fi
    return
  fi

  # file: existence check
  local file
  file="$(yq ".stacks.${stack}.extras[${idx}].file // \"\"" "$STACKS_YML")"
  if [[ -n "$file" ]]; then
    if [[ -f "$loc/$file" ]]; then echo "$token"; fi
    return
  fi

  # grep: + in:
  local pattern
  pattern="$(yq ".stacks.${stack}.extras[${idx}].grep // \"\"" "$STACKS_YML")"
  if [[ -n "$pattern" ]]; then
    local grep_files
    mapfile -t grep_files < <(yq ".stacks.${stack}.extras[${idx}].in[]" "$STACKS_YML" 2>/dev/null || true)
    for f in "${grep_files[@]}"; do
      if [[ -f "$loc/$f" ]] && grep -qE "$pattern" "$loc/$f" 2>/dev/null; then
        echo "$token"
        return
      fi
    done
    return
  fi

  # any_of: OR over sub-rules (supports file: and grep:+in: sub-rules)
  local any_of_count
  any_of_count="$(yq ".stacks.${stack}.extras[${idx}].any_of | length" "$STACKS_YML" 2>/dev/null || echo 0)"
  if ((any_of_count > 0)); then
    local i
    for ((i = 0; i < any_of_count; i++)); do
      local sub_file sub_pattern
      sub_file="$(yq ".stacks.${stack}.extras[${idx}].any_of[${i}].file // \"\"" "$STACKS_YML")"
      if [[ -n "$sub_file" ]] && [[ -f "$loc/$sub_file" ]]; then
        echo "$token"
        return
      fi
      sub_pattern="$(yq ".stacks.${stack}.extras[${idx}].any_of[${i}].grep // \"\"" "$STACKS_YML")"
      if [[ -n "$sub_pattern" ]]; then
        local sub_files
        mapfile -t sub_files < <(yq ".stacks.${stack}.extras[${idx}].any_of[${i}].in[]" "$STACKS_YML" 2>/dev/null || true)
        for f in "${sub_files[@]}"; do
          if [[ -f "$loc/$f" ]] && grep -qE "$sub_pattern" "$loc/$f" 2>/dev/null; then
            echo "$token"
            return
          fi
        done
      fi
    done
  fi
}

# Early exit: skip repos with no stack sentinels at all.
# Uses find_stack_dir (which walks search_dirs) rather than a flat $ROOT/
# prefix check -- sentinel files may live in subdirectories.
has_stack=0
for stack in "${STACK_NAMES[@]}"; do
  if [[ -n "$(find_stack_dir "$stack")" ]]; then
    has_stack=1
    break
  fi
done
((has_stack)) || exit 0

echo "root: $ROOT"

js_loc=""

for stack in "${STACK_NAMES[@]}"; do
  loc="$(find_stack_dir "$stack")"
  [[ -z "$loc" ]] && continue

  # Evaluate extras for this stack
  extra_count="$(yq ".stacks.${stack}.extras | length" "$STACKS_YML" 2>/dev/null || echo 0)"
  extras_parts=()
  for ((i = 0; i < extra_count; i++)); do
    matched="$(eval_extra "$stack" "$loc" "$i")"
    [[ -n "$matched" ]] && extras_parts+=("$matched")
  done

  # Package manager suffix (js only)
  suffix=""
  if [[ "$stack" == "js" ]]; then
    pm="$(resolve_package_manager "$loc")"
    suffix="[${pm:-npm}]"
    js_loc="$loc"
  fi

  local_IFS="${IFS:-}"
  IFS=', '
  parts_str="${extras_parts[*]:-}"
  IFS="$local_IFS"

  emit_stack "$stack" "$loc" "$parts_str" "$suffix"
done

# Node version. Prefer the JS stack's location, fall back to project root.
if [[ -n "$js_loc" ]]; then
  if [[ -f "$js_loc/.nvmrc" ]]; then
    echo "node: $(tr -d 'v\n' <"$js_loc/.nvmrc")"
  elif [[ -f .nvmrc ]]; then
    echo "node: $(tr -d 'v\n' <.nvmrc)"
  fi
fi
