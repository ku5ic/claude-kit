#!/usr/bin/env bash
# SessionStart hook. Prepends repo context at session start/resume/compact/
# clear. The harness itself guarantees this fires once per boundary (matcher:
# startup|resume|compact|clear in settings.json) -- no self-dedup needed here.
HOOK_NAME="inject-context.sh"
# shellcheck source=_lib.sh
source "$(dirname "$0")/_lib.sh"
# shellcheck source=../bin/_lib.sh
source "$(dirname "$0")/../bin/_lib.sh"

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
  echo "</repo-context>"
fi

# Only global_skills are emitted here; stack-derived skills go through
# emit_suggested_skills so session start only blocks on the core set. Not
# gated on the stack cache being non-empty: global_skills apply to every
# project regardless of detected stack (a docs-only or config-only repo
# still deserves them - the earlier `project_name` exit above already
# filters out non-project contexts).
emit_required_skills() {
  local yml="$KIT_ROOT/_stacks.yml"

  [[ -f "$yml" ]] || return 0
  command -v yq >/dev/null 2>&1 || return 0

  local -a required=()
  mapfile -t required < <(global_skills_list "$yml")
  [[ ${#required[@]} -eq 0 ]] && return 0

  render_required_skills_block "$yml"

  if command -v jq >/dev/null 2>&1; then
    local log_dir ts sk
    log_dir="$HOME/.claude/logs"
    mkdir -p "$log_dir"
    ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    for sk in "${required[@]}"; do
      jq -cn \
        --arg ts "$ts" \
        --arg sess "$session_id" \
        --arg cwd "$cwd" \
        --arg skill "$sk" \
        '{ts:$ts,hook:"inject-context.sh",event:"required-skill",session_id:$sess,cwd:$cwd,expansion_type:null,command_name:null,command_args:null,command_source:null,skill_file:$skill,tool_name:null}' \
        >>"$log_dir/skills.jsonl"
    done
  fi
}

# Skills already in global_skills are excluded (required, not suggested).
# Logged as event:"suggested-skill" so skills-report.sh can measure whether a
# suggestion was ever acted on - this records "surfaced", not "loaded";
# log-skills.sh's Skill-tool/Read entries are the only record of an
# actual invocation.
emit_suggested_skills() {
  local cache="$1"
  local yml="$KIT_ROOT/_stacks.yml"

  [[ -s "$cache" ]] || return 0
  [[ -f "$yml" ]] || return 0
  command -v yq >/dev/null 2>&1 || return 0

  local -a suggested=()
  mapfile -t suggested < <(stacks_signals_from_cache "$cache" | suggested_skills_from_signals "$yml")
  [[ ${#suggested[@]} -eq 0 ]] && return 0

  render_suggested_skills_block "$yml" "$cache"

  local sk
  if command -v jq >/dev/null 2>&1; then
    local log_dir ts
    log_dir="$HOME/.claude/logs"
    mkdir -p "$log_dir"
    ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    for sk in "${suggested[@]}"; do
      jq -cn \
        --arg ts "$ts" \
        --arg sess "$session_id" \
        --arg cwd "$cwd" \
        --arg skill "$sk" \
        '{ts:$ts,hook:"inject-context.sh",event:"suggested-skill",session_id:$sess,cwd:$cwd,expansion_type:null,command_name:null,command_args:null,command_source:null,skill_file:$skill,tool_name:null}' \
        >>"$log_dir/skills.jsonl"
    done
  fi
}

# Emits a <tooling> block computed live (not from the stack cache) for JS/TS
# and Python projects. Root-and-workspace-level only; search_dirs
# subdirectories aren't walked (same scope as the sentinel walk).
emit_tooling_block() {
  local root="$1"

  local has_js=0 has_python=0
  [[ -f "$root/package.json" ]] && has_js=1
  if [[ -f "$root/pyproject.toml" || -f "$root/requirements.txt" || -f "$root/Pipfile" ]]; then
    has_python=1
  fi
  if ! ((has_js)) && ! ((has_python)); then
    return 0
  fi

  local pm
  pm="$(resolve_package_manager "$root" 2>/dev/null || true)"

  local body
  body="$(
    set +e

    if ((has_js)) && command -v jq >/dev/null 2>&1; then
      [[ -n "$pm" ]] && echo "package-manager: $pm"

      if jq -e 'has("scripts") and (.scripts | length > 0)' "$root/package.json" >/dev/null 2>&1; then
        echo "scripts (package.json):"
        jq -r '.scripts | to_entries[] | "  \(.key): \(.value[0:120])\(if (.value | length) > 120 then "..." else "" end)"' "$root/package.json" 2>/dev/null
      fi

      local is_ws=0
      if jq -e 'has("workspaces")' "$root/package.json" >/dev/null 2>&1 || [[ -f "$root/pnpm-workspace.yaml" ]]; then
        is_ws=1
      fi

      if ((is_ws)); then
        local -a ws_pats=()
        if jq -e '.workspaces | arrays' "$root/package.json" >/dev/null 2>&1; then
          mapfile -t ws_pats < <(jq -r '.workspaces[]' "$root/package.json" 2>/dev/null)
        elif jq -e '.workspaces.packages | arrays' "$root/package.json" >/dev/null 2>&1; then
          mapfile -t ws_pats < <(jq -r '.workspaces.packages[]' "$root/package.json" 2>/dev/null)
        fi
        if command -v yq >/dev/null 2>&1 && [[ -f "$root/pnpm-workspace.yaml" ]]; then
          local -a _pp
          mapfile -t _pp < <(yq '.packages[]' "$root/pnpm-workspace.yaml" 2>/dev/null)
          ws_pats+=("${_pp[@]}")
        fi
        if [[ ${#ws_pats[@]} -gt 0 ]]; then
          (
            cd "$root" 2>/dev/null || exit 0
            shopt -s nullglob globstar 2>/dev/null || true
            local _pat _dir _sc _ws_shown=0 _ws_max=20
            for _pat in "${ws_pats[@]}"; do
              # shellcheck disable=SC2231
              for _dir in $_pat; do
                [[ -d "$_dir" && -f "$_dir/package.json" ]] || continue
                _sc="$(jq '.scripts | length' "$_dir/package.json" 2>/dev/null || echo 0)"
                [[ "${_sc:-0}" -gt 0 ]] || continue
                if [[ $_ws_shown -ge $_ws_max ]]; then
                  echo "  (workspace packages capped at $_ws_max; run-checks.sh covers all)"
                  break 2
                fi
                echo "scripts ($_dir/package.json):"
                jq -r '.scripts | to_entries[] | "  \(.key): \(.value[0:120])\(if (.value | length) > 120 then "..." else "" end)"' "$_dir/package.json" 2>/dev/null
                _ws_shown=$((_ws_shown + 1))
              done
            done
          ) 2>/dev/null
        fi
      fi
    fi

    if ((has_python)) && ! ((has_js)) && [[ -n "$pm" ]]; then
      echo "package-manager: $pm"
      case "$pm" in
      uv) echo "run-form: uv run <command>" ;;
      poetry) echo "run-form: poetry run <command>" ;;
      pipenv) echo "run-form: pipenv run <command>" ;;
      esac
    fi

    if ((has_python)) && [[ -f "$root/Makefile" ]]; then
      local _mkt
      _mkt="$(grep -E '^[a-zA-Z][a-zA-Z0-9_-]*[[:space:]]*:' "$root/Makefile" 2>/dev/null | cut -d: -f1 | tr -d '[:space:]' | sort -u || true)"
      if [[ -n "$_mkt" ]]; then
        echo "makefile-targets:"
        printf '%s\n' "$_mkt" | sed 's/^/  /'
      fi
    fi

    if ((has_python)); then
      local _jf=""
      [[ -f "$root/justfile" ]] && _jf="$root/justfile"
      [[ -f "$root/Justfile" ]] && _jf="$root/Justfile"
      if [[ -n "$_jf" ]]; then
        local _jft
        _jft="$(grep -E '^[a-zA-Z_][a-zA-Z0-9_-]*' "$_jf" 2>/dev/null | grep -v '^#' | cut -d: -f1 | sed 's/[[:space:]].*//' | sort -u || true)"
        if [[ -n "$_jft" ]]; then
          echo "justfile-targets:"
          printf '%s\n' "$_jft" | sed 's/^/  /'
        fi
      fi
    fi

    true
  )"

  [[ -z "$body" ]] && return 0

  echo ""
  echo "<tooling>"
  printf '%s\n' "$body"
  echo ""
  echo "guidance: Run scripts only through the package manager named above, prefer these scripts and run-checks.sh over direct tool invocation, and never substitute a different package manager."
  echo "</tooling>"
}

emit_required_skills

emit_suggested_skills "$cache_file"

emit_tooling_block "$project_root"

exit 0
