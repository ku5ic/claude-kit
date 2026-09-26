#!/usr/bin/env bash
# SessionStart hook. Prepends repo context at session start/resume/compact/
# clear. The harness itself guarantees this fires once per boundary (matcher:
# startup|resume|compact|clear in settings.json) -- no self-dedup needed here.
HOOK_NAME="inject-context.sh"

# shellcheck source=../bin/_lib.sh
source "$(dirname "$0")/../bin/_lib.sh"

# Every guard fails open without these, so say so at session start. The JSON
# is built with printf because jq may be the thing missing. Under bash 3.2
# the lib stopped at its version gate, and this is all that runs.
missing=""
check_prereqs || missing="$KIT_PREREQ_MISSING"
if ! check_install; then
  missing="${missing:+$missing; }$KIT_PREREQ_MISSING"
fi
if [ -n "$missing" ]; then
  printf '{"systemMessage":"claude-kit guards fail open until this is fixed. Missing: %s"}\n' "$missing"
  exit 0
fi

kit_hook_init

payload=""
read_payload
session_id="$(printf '%s' "$payload" | jq -r '.session_id // empty' 2>/dev/null || true)"
cwd="$(printf '%s' "$payload" | jq -r '.cwd // empty' 2>/dev/null || true)"

project_name="$("$KIT_ROOT/bin/project-name.sh" 2>/dev/null || echo "unknown")"

# Skip injection for non-project contexts.
case "$project_name" in
home | root | unknown)
  exit 0
  ;;
esac

project_root="$("$KIT_ROOT/bin/project-root.sh" 2>/dev/null || echo "")"
[[ -z "$project_root" ]] && exit 0

cache_file="$(stack_cache_file "$project_name" "$project_root")"
refresh_stack_cache_if_stale "$project_root" "$cache_file"

if [[ -s "$cache_file" ]]; then
  echo ""
  echo "<repo-context>"
  cat "$cache_file"
  echo "branch (at session start): $(git -C "$project_root" branch --show-current 2>/dev/null || echo unknown)"
  dirty="$(git -C "$project_root" status --porcelain 2>/dev/null | wc -l | tr -d ' ')" || dirty="unknown"
  echo "dirty-files (at session start): $dirty"
  # --no-create: a session that never writes a report leaves no scratch dir.
  echo "scratch: $(cd "$project_root" && kit_dir scratch --no-create)"
  echo "</repo-context>"
fi

# Only global_skills are emitted here; stack-derived skills go through
# emit_suggested_skills so session start only blocks on the core set. Not
# gated on the stack cache being non-empty: global_skills apply to every
# project regardless of detected stack (a docs-only or config-only repo
# still deserves them - the earlier `project_name` exit above already
# filters out non-project contexts).
emit_required_skills() {
  local yml="$KIT_YML"

  [[ -f "$yml" ]] || return 0
  command -v yq >/dev/null 2>&1 || return 0

  local -a required=()
  mapfile -t required < <(global_skills_list "$yml")
  [[ ${#required[@]} -eq 0 ]] && return 0

  render_required_skills_block "$yml"

  local sk
  for sk in "${required[@]}"; do
    log_event skills required-skill cwd "$cwd" skill_file "$sk"
  done
}

# Skills already in global_skills are excluded (required, not suggested).
# Logged as event:"suggested-skill" so skills-report.sh can measure whether a
# suggestion was ever acted on - this records "surfaced", not "loaded";
# log-skills.sh's Skill-tool/Read entries are the only record of an
# actual invocation.
emit_suggested_skills() {
  local cache="$1"
  local yml="$KIT_YML"

  [[ -s "$cache" ]] || return 0
  [[ -f "$yml" ]] || return 0
  command -v yq >/dev/null 2>&1 || return 0

  local -a suggested=()
  mapfile -t suggested < <(stacks_signals_from_cache "$cache" | suggested_skills_from_signals "$yml")
  [[ ${#suggested[@]} -eq 0 ]] && return 0

  render_suggested_skills_block "$yml" "$cache"

  local sk
  for sk in "${suggested[@]}"; do
    log_event skills suggested-skill cwd "$cwd" skill_file "$sk"
  done
}

# Emits a <tooling> block computed live (not from the stack cache): for each
# subproject, the run form of every task kit.yml's task_providers find and of
# every toolchain check its stacks get. The same lists run-checks.sh reads, so
# Claude is never told about a task the checks skip, or the reverse.
emit_tooling_block() {
  local root="$1"

  local body
  body="$(
    set +e
    local pm sub dir header shown=0 max=20 i
    local -a lines
    kit_stacks_load
    pm="$(resolve_package_manager "$root" 2>/dev/null)"
    [[ -n "$pm" ]] && echo "package-manager: $pm"

    while IFS= read -r sub; do
      lines=()
      dir="$root"
      header="tasks:"
      if [[ "$sub" != . ]]; then
        dir="$root/$sub"
        header="tasks [$sub]:"
      fi
      mapfile -t lines < <(kit_tasks "$dir" | cut -f4)
      for ((i = 0; i < ${#KIT_TC_STACKS[@]}; i++)); do
        if kit_dir_has_stack "$dir" "${KIT_TC_STACKS[i]}" && kit_toolchain_cmd "$i" "$dir"; then
          lines+=("$KIT_TC_CMD")
        fi
      done
      ((${#lines[@]} > 0)) || continue
      if [[ "$sub" != . ]]; then
        if ((shown >= max)); then
          echo "(subprojects capped at $max; run-checks.sh covers all)"
          break
        fi
        shown=$((shown + 1))
      fi
      echo "$header"
      printf '  %s\n' "${lines[@]}"
    done < <(kit_subprojects "$root")
    true
  )"

  # kit.yml's tools, split by whether they're on PATH: what is installed is
  # what PATH says, not what any package list claims.
  local tool tools_body
  local -a available=() missing=()
  kit_stacks_load
  for tool in "${KIT_TOOLS[@]}"; do
    if command -v "$tool" >/dev/null 2>&1; then
      available+=("$tool")
    else
      missing+=("$tool")
    fi
  done
  tools_body="$(
    IFS=,
    ((${#available[@]} > 0)) && echo "available: ${available[*]}"
    ((${#missing[@]} > 0)) && echo "missing: ${missing[*]}"
    true
  )"
  tools_body="${tools_body//,/, }"

  [[ -z "$body" && -z "$tools_body" ]] && return 0

  echo ""
  echo "<tooling>"
  [[ -n "$body" ]] && printf '%s\n' "$body"
  [[ -n "$tools_body" ]] && printf '%s\n' "$tools_body"
  if [[ -n "$body" ]]; then
    echo ""
    echo "guidance: Run scripts only through the package manager named above, prefer these scripts and run-checks.sh over direct tool invocation, and never substitute a different package manager."
  fi
  echo "</tooling>"
}

emit_required_skills

emit_suggested_skills "$cache_file"

emit_tooling_block "$project_root"

exit 0
