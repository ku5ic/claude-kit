#!/usr/bin/env bash
# Lists the files that import a given file: the blast radius of editing it
# (rules/change.md section 3).
#
#   blast-radius.sh <file> [symbol]
#
# JS/TS: import, export-from, require() and import() specifiers. Relative
# specifiers resolve against the importing file; "@/" and "~/" aliases match
# on the path suffix (alias-match); a workspace package's name, or
# name/subpath, counts for every file in that package (workspace-match).
# Python: "import m" and "from m import x", absolute or relative. The module
# path follows the __init__.py chain, else it is root-relative minus src/.
# With [symbol], only consumers using it as a whole word are kept.
#
# Prints counts, then up to 50 "path:line <source|test> [label]" lines; test
# is decided by the test-patterns globs in kit.yml. Scans the repo's tracked
# and untracked-but-not-ignored files. Static only: a non-literal import() or
# require() prints "unresolvable imports present".

set -euo pipefail

# shellcheck source=_lib.sh
source "$(dirname "$0")/_lib.sh"

if (($# < 1 || $# > 2)); then
  echo "usage: blast-radius.sh <file> [symbol]" >&2
  exit 2
fi
target="$1" symbol="${2:-}"
if [[ ! -f "$target" ]]; then
  echo "blast-radius: no such file: $target" >&2
  exit 2
fi
abs="$(cd -P "$(dirname "$target")" && pwd)/${target##*/}"
if ! root="$(git -C "${abs%/*}" rev-parse --show-toplevel 2>/dev/null)"; then
  echo "blast-radius: not inside a git repository" >&2
  exit 2
fi
rel="${abs#"$root"/}"

JS_FILES=('*.js' '*.jsx' '*.ts' '*.tsx' '*.mjs' '*.cjs' '*.mts' '*.cts' '*.vue' '*.svelte' '*.astro')

# grep_files <ERE> <pathspec>...: "path:line:content" for matching lines.
grep_files() {
  local re="$1"
  shift
  git -C "$root" ls-files -z --cached --others --exclude-standard -- "$@" |
    (cd "$root" && xargs -0 grep -snHE -- "$re" /dev/null) || true
}

# Prints $1 with "." and ".." segments resolved; "" for the root.
norm_path() {
  local IFS=/ part
  local -a out=()
  for part in $1; do
    case "$part" in
    '' | .) ;;
    ..) if ((${#out[@]})); then unset "out[${#out[@]}-1]"; fi ;;
    *) out+=("$part") ;;
    esac
  done
  printf '%s\n' "${out[*]}"
}

path_parent() {
  if [[ "$1" == */* ]]; then printf '%s\n' "${1%/*}"; fi
}

dotted_parent() {
  if [[ "$1" == *.* ]]; then printf '%s\n' "${1%.*}"; fi
}

strip_js_ext() {
  case "$1" in
  *.js | *.jsx | *.ts | *.tsx | *.mjs | *.cjs | *.mts | *.cts) printf '%s\n' "${1%.*}" ;;
  *) printf '%s\n' "$1" ;;
  esac
}

# Dotted module name of root-relative .py path $1.
py_module() {
  local path="$1" dir mod pkgroot
  dir="$(path_parent "$path")"
  mod="${path%.py}"
  mod="${mod%/__init__}"
  if [[ -n "$dir" && -f "$root/$dir/__init__.py" ]]; then
    pkgroot="$dir"
    while [[ -n "$pkgroot" && -f "$root/$pkgroot/__init__.py" ]]; do
      pkgroot="$(path_parent "$pkgroot")"
    done
    if [[ -n "$pkgroot" ]]; then mod="${mod#"$pkgroot"/}"; fi
  else
    mod="${mod#src/}"
  fi
  printf '%s\n' "${mod//\//.}"
}

# Absolute module for "from <$2> import" written in file $1.
py_resolve() {
  local file="$1" spec="$2" dots pkg i
  if [[ "$spec" != .* ]]; then
    printf '%s\n' "$spec"
    return 0
  fi
  dots="${spec%%[!.]*}"
  spec="${spec#"$dots"}"
  pkg="$(py_module "$file")"
  if [[ "${file##*/}" != __init__.py ]]; then pkg="$(dotted_parent "$pkg")"; fi
  for ((i = 1; i < ${#dots}; i++)); do pkg="$(dotted_parent "$pkg")"; done
  if [[ -n "$pkg" && -n "$spec" ]]; then
    printf '%s.%s\n' "$pkg" "$spec"
  else
    printf '%s\n' "$pkg$spec"
  fi
}

test_globs=()
while IFS= read -r glob; do
  if [[ -n "$glob" ]]; then test_globs+=("$glob"); fi
done < <(yq '.skill_file_map[] | select(.skills[] == "test-patterns") | .globs[]' "$KIT_YML" 2>/dev/null || true)

is_test() {
  local base="${1##*/}" glob
  for glob in "${test_globs[@]}"; do
    # shellcheck disable=SC2053  # the glob is meant to match
    if [[ "$base" == $glob ]]; then return 0; fi
  done
  return 1
}

declare -A seen=()
results=()
tests=0

# add_consumer <file> <line> [label]
add_consumer() {
  [[ "$1" != "$rel" && -z "${seen[$1]:-}" ]] || return 0
  seen[$1]=1
  if [[ -n "$symbol" ]] && ! grep -qw -- "$symbol" "$root/$1"; then return 0; fi
  local kind="source"
  if is_test "$1"; then
    kind="test"
    tests=$((tests + 1))
  fi
  results+=("$1:$2 $kind${3:+ $3}")
}

# Lines of <ERE> matches, narrowed to lines containing one of the needles.
candidate_lines() {
  local re="$1"
  shift
  local -a needles=() specs=()
  while (($#)) && [[ "$1" != -- ]]; do
    if [[ -n "$1" ]]; then needles+=(-e "$1"); fi
    shift
  done
  shift
  specs=("$@")
  if ((${#needles[@]})); then
    grep_files "$re" "${specs[@]}" | { grep -F "${needles[@]}" || true; }
  else
    grep_files "$re" "${specs[@]}"
  fi
}

dynamic=0
case "$rel" in
*.js | *.jsx | *.ts | *.tsx | *.mjs | *.cjs | *.mts | *.cts)
  stem="$(strip_js_ext "$rel")"
  index_dir=""
  is_index=0
  if [[ "${stem##*/}" == index ]]; then
    is_index=1
    index_dir="$(path_parent "$stem")"
  fi

  ws_names=()
  while IFS= read -r dir; do
    if [[ "$dir" != . && "$rel" == "$dir"/* && -f "$root/$dir/package.json" ]]; then
      name="$(jq -r '.name // empty' "$root/$dir/package.json" 2>/dev/null || true)"
      if [[ -n "$name" ]]; then ws_names+=("$name"); fi
    fi
  done < <(kit_subprojects "$root")

  spec_re="(from|import|require[[:space:]]*\\(|import[[:space:]]*\\()[[:space:]]*['\"]([^'\"]+)['\"]"
  # An index is also reached by a bare '.', '..', or './' that names nothing.
  index_needles=()
  if ((is_index)); then index_needles=(".'" '."' "/'" '/"'); fi
  while IFS= read -r hit; do
    file="${hit%%:*}"
    rest="${hit#*:}"
    line="${rest%%:*}"
    content="${rest#*:}"
    while [[ "$content" =~ $spec_re ]]; do
      spec="${BASH_REMATCH[2]}"
      content="${content#*"${BASH_REMATCH[0]}"}"
      case "$spec" in
      .*)
        resolved="$(strip_js_ext "$(norm_path "$(path_parent "$file")/$spec")")"
        if [[ "$resolved" == "$stem" ]] || { ((is_index)) && [[ "$resolved" == "$index_dir" ]]; }; then
          add_consumer "$file" "$line"
        fi
        ;;
      @/* | \~/*)
        suffix="$(strip_js_ext "${spec#?/}")"
        if [[ "$stem" == "$suffix" || "$stem" == */"$suffix" ]] ||
          { ((is_index)) && [[ "$index_dir" == "$suffix" || "$index_dir" == */"$suffix" ]]; }; then
          add_consumer "$file" "$line" alias-match
        fi
        ;;
      *)
        for name in "${ws_names[@]}"; do
          if [[ "$spec" == "$name" || "$spec" == "$name"/* ]]; then
            add_consumer "$file" "$line" workspace-match
          fi
        done
        ;;
      esac
    done
  done < <(candidate_lines "(from|import|require)[[:space:]]*\\(?[[:space:]]*['\"]" \
    "${stem##*/}" "${index_dir##*/}" "${ws_names[@]}" "${index_needles[@]}" -- "${JS_FILES[@]}")

  if [[ -n "$(grep_files "(^|[^A-Za-z0-9_\$.])(import|require)[[:space:]]*\\([[:space:]]*[^\"'[:space:])]" "${JS_FILES[@]}")" ]]; then
    dynamic=1
  fi
  ;;
*.py)
  mod="$(py_module "$rel")"
  mod_parent="$(dotted_parent "$mod")"
  leaf="${mod##*.}"
  from_re='^[[:space:]]*from[[:space:]]+([.A-Za-z0-9_]+)[[:space:]]+import[[:space:]]+(.*)$'
  import_re='^[[:space:]]*import[[:space:]]+(.*)$'
  while IFS= read -r hit; do
    file="${hit%%:*}"
    rest="${hit#*:}"
    line="${rest%%:*}"
    content="${rest#*:}"
    content="${content%%#*}"
    if [[ "$content" =~ $from_re ]]; then
      names=" ${BASH_REMATCH[2]//[(),]/ } "
      from="$(py_resolve "$file" "${BASH_REMATCH[1]}")"
      if [[ -n "$from" ]] && [[ "$from" == "$mod" || ("$from" == "$mod_parent" && "$names" == *[[:space:]]"$leaf"[[:space:]]*) ]]; then
        add_consumer "$file" "$line"
      fi
    elif [[ "$content" =~ $import_re ]]; then
      IFS=, read -ra items <<<"${BASH_REMATCH[1]}"
      for item in "${items[@]}"; do
        read -r item _ <<<"$item"
        if [[ "$item" == "$mod" || "$item" == "$mod".* ]]; then
          add_consumer "$file" "$line"
        fi
      done
    fi
  done < <(candidate_lines '^[[:space:]]*(from[[:space:]]+[.A-Za-z0-9_]+[[:space:]]+import[[:space:]]|import[[:space:]]+[A-Za-z_])' \
    "$leaf" -- '*.py')
  ;;
*)
  echo "blast-radius: no import scanner for ${rel##*/}" >&2
  exit 2
  ;;
esac

printf 'blast-radius: %s%s\n' "$rel" "${symbol:+ (symbol $symbol)}"
printf 'consumers: %d (source %d, test %d)\n' "${#results[@]}" $((${#results[@]} - tests)) "$tests"
if ((${#results[@]})); then
  printf '%s\n' "${results[@]}" | sort | awk 'NR <= 50'
fi
if ((${#results[@]} > 50)); then
  printf '... %d more\n' $((${#results[@]} - 50))
fi
if ((dynamic)); then
  echo "unresolvable imports present"
fi
