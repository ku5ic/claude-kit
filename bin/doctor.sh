#!/usr/bin/env bash
# Verifies the Claude config: symlink layout AND cross-file consistency.
# Used by .github/workflows/lint.yml and runnable locally.
#
# Checks:
#   1. Symlinks: each top-level claude/ entry is symlinked to the dotfiles
#      source. Verifies link existence and target path.
#   2. Credential pattern parity: settings.json's deny rules mention every
#      kit.yml sensitive_paths entry. The deny rules cover Read/Edit;
#      guard-bash.sh reads the same list for what permissions cannot express
#      (cat, cp of a key file).
#   3. Agent-context / inject-context derivation parity: both consumers share
#      the yq derivation queries via bin/_lib.sh instead of holding private
#      copies.
#   4. Skill and agent frontmatter lint: every procedure SKILL.md and
#      agents/*.md has a valid model field, no dated model pin, a matching
#      effort field, and no pin that just restates the session default in
#      settings.json.
#   5. Skill map validation: skill_file_map and skill_triggers reference only
#      skills that exist, and every stack/extra skill has a trigger entry.
#   6. Skills-log field parity: log-skills.sh and skills-report.sh reference
#      the same skills.jsonl field names, so a rename in the emitter cannot
#      silently break the report.
#   7. Audit-verify field parity: audit/references/verify.md's per-finding parser
#      references the same field names as the report-format skill's required
#      per-finding shape.
#   8. CLAUDE.md rules pointer parity: every rules/*.md reference in
#      CLAUDE.md resolves to a real file, catching a stale pointer left
#      behind by a rename or delete.
#   9. settings.json machine-local leak: autoMode.environment (machine-
#      appended per-repo context) never regrows the org-internal detail
#      stripped from it once already.
#   10. MCP allow-list server parity: reports which mcp__ permission entries
#       name a server actually configured here (`claude mcp list`). Advisory
#       only, never a failure -- settings.json is shared across machines with
#       different connector sets, so a rule for a server absent locally is
#       inert rather than stale, and `claude mcp list` is local-only so the
#       two cannot be told apart. Plugin-provided servers (`plugin_*`) are
#       unverifiable for a different reason: they live in a plugin's own
#       .mcp.json, which `claude mcp list` never shows. The tool segment is
#       not validated at all; the CLI exposes no way to enumerate a server's
#       tools.
#
# Adding a credential pattern: add it to kit.yml's sensitive_paths AND to
# settings.json's deny array.
#
# Exit codes: 0 = all checks passed, 1 = one or more checks failed.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
SOURCE_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# Dotfiles layout: personal settings, CLAUDE.md, and rules live beside the kit.
PERSONAL_ROOT="$(cd "$SOURCE_ROOT/../claude" && pwd)"
# shellcheck source=_lib.sh
source "$SCRIPT_DIR/_lib.sh"
TARGET_ROOT="$KIT_HOME"

ENTRIES=(settings.json CLAUDE.md hooks skills agents rules bin kit.yml claude-kit.local.yml)

root_for() {
  case "$1" in
  settings.json | CLAUDE.md | rules | claude-kit.local.yml) echo "$PERSONAL_ROOT" ;;
  *) echo "$SOURCE_ROOT" ;;
  esac
}

exit_code=0

# Every check below needs these; without them, stop here instead of reporting
# a pile of downstream failures.
echo "== prerequisites =="
if ! check_prereqs; then
  echo "missing        $KIT_PREREQ_MISSING"
  exit 1
fi
echo "ok             bash 4.4+, jq, and mikefarah yq present"
echo

# CI runners have no ~/.claude install, so symlink targets never resolve correctly.
if [[ "${CI:-}" == "true" ]]; then
  echo "== symlinks == (skipped: running in CI)"
elif [[ -d "$TARGET_ROOT" ]]; then
  echo "== symlinks =="
  for entry in "${ENTRIES[@]}"; do
    src="$(root_for "$entry")/$entry"
    dst="$TARGET_ROOT/$entry"

    if [[ ! -L "$dst" ]]; then
      if [[ -e "$dst" ]]; then
        echo "not-a-symlink  $dst"
      else
        echo "missing        $dst"
      fi
      exit_code=1
      continue
    fi

    actual="$(readlink "$dst")"
    if [[ "${actual%/}" != "$src" ]]; then
      echo "wrong-target   $dst -> $actual (expected $src)"
      exit_code=1
      continue
    fi

    echo "ok             $dst"
  done
else
  echo "== symlinks == (skipped: $TARGET_ROOT does not exist)"
fi

echo
echo "== credential pattern parity =="

SETTINGS="$PERSONAL_ROOT/settings.json"

# kit.yml's "~/..." entries are compared by their path tail, which is what
# settings.json's "~/..." rules contain too.
mapfile -t patterns < <(yq '.sensitive_paths // [] | .[]' "$SOURCE_ROOT/kit.yml" 2>/dev/null)
parity_failed=0
if ((${#patterns[@]} == 0)); then
  echo "empty          kit.yml has no sensitive_paths"
  parity_failed=1
fi
for pat in "${patterns[@]}"; do
  pat="${pat#\~/}"
  if ! grep -qF "$pat" "$SETTINGS"; then
    echo "missing-pattern  settings.json: '$pat'"
    parity_failed=1
  fi
done

if ((parity_failed)); then
  exit_code=1
else
  echo "ok             ${#patterns[@]} kit.yml sensitive_paths mirrored in settings.json"
fi

echo
echo "== agent-context / inject-context derivation parity =="

AGENT_CONTEXT="$SOURCE_ROOT/bin/agent-context.sh"
INJECT_CONTEXT="$SOURCE_ROOT/hooks/inject-context.sh"
derivation_failed=0

if [[ ! -f "$AGENT_CONTEXT" ]]; then
  echo "missing        $AGENT_CONTEXT"
  derivation_failed=1
elif ! grep -qF '_lib.sh' "$AGENT_CONTEXT"; then
  echo "no-source      agent-context.sh does not source bin/_lib.sh"
  derivation_failed=1
fi

if [[ ! -f "$INJECT_CONTEXT" ]]; then
  echo "missing        $INJECT_CONTEXT"
  derivation_failed=1
elif ! grep -qF '_lib.sh' "$INJECT_CONTEXT"; then
  echo "no-source      inject-context.sh does not source bin/_lib.sh"
  derivation_failed=1
fi

# Neither consumer may hold a private copy of the shared yq derivation
# queries; those must live only in bin/_lib.sh (global_skills_list,
# stacks_signals_from_cache, suggested_skills_from_signals).
# These are literal grep -qF patterns: the \$ is escaped so the string holds
# the verbatim ${stack}/${sig} text to search for, not a value to expand.
private_copy_patterns=(
  ".global_skills"
  ".stacks.\${stack}.extras"
  ".stacks.\${sig}.skills"
)
for pat in "${private_copy_patterns[@]}"; do
  for f in "$AGENT_CONTEXT" "$INJECT_CONTEXT"; do
    [[ -f "$f" ]] || continue
    if grep -qF "$pat" "$f"; then
      echo "private-copy   $f queries '$pat' directly instead of using bin/_lib.sh"
      derivation_failed=1
    fi
  done
done

if ((derivation_failed)); then
  exit_code=1
else
  echo "ok             agent-context.sh and inject-context.sh share derivation via bin/_lib.sh"
fi

echo
echo "== skill + agent frontmatter lint =="

SKILLS_DIR="$SOURCE_ROOT/skills"
AGENTS_DIR="$SOURCE_ROOT/agents"

# Session defaults a pin is measured against (see the "pins at most one
# field: the one that diverges from the session default" rule below). Not
# opusplan-aware: opusplan resolves to opus in plan mode and sonnet in
# execution, so a bare "opus"/"sonnet" pin is a deliberate phase override,
# not a redundant one. Only a literal match to the raw settings.json value
# counts - this catches the effort-pin case (Problem 3) now and the
# straight-model case (e.g. plain "sonnet") on any future switch back.
session_model=$(jq -r '.model // ""' "$SETTINGS" 2>/dev/null || true)
session_effort=$(jq -r '.effortLevel // ""' "$SETTINGS" 2>/dev/null || true)
fm_failed=0
fm_count=0

while IFS= read -r f; do
  # Extract YAML frontmatter between the first pair of --- delimiters.
  fm=$(awk 'NR==1 && /^---$/{in_fm=1;next} in_fm && /^---$/{exit} in_fm{print}' "$f")
  rel="${f#"$SOURCE_ROOT/"}"

  if [[ -z "$fm" ]]; then
    echo "missing-frontmatter  $rel"
    fm_failed=1
    continue
  fi

  # Only procedure skills (migrated from commands) and agents carry model
  # and/or effort; patterns and reference skills have neither. Lint the
  # former, skip the latter.
  if ! printf '%s\n' "$fm" | grep -qE '^(model|effort):'; then
    continue
  fi
  fm_count=$((fm_count + 1))

  # Filter to just model/effort lines before passing to yq: other frontmatter
  # fields (e.g. argument-hint: <...>) contain angle brackets that yq rejects
  # as invalid YAML.
  fm_safe=$(printf '%s\n' "$fm" | grep -E '^(model|effort):')
  model=$(printf '%s\n' "$fm_safe" | yq '.model // ""' 2>/dev/null || true)
  effort=$(printf '%s\n' "$fm_safe" | yq '.effort // ""' 2>/dev/null || true)
  # yq may emit the literal string "null" for absent keys; normalize to empty.
  [[ "$model" == "null" ]] && model=""
  [[ "$effort" == "null" ]] && effort=""

  # Each skill/agent pins at most one field: the one that diverges from the
  # session default. An empty model or empty effort here is intentional,
  # not missing.
  if [[ -n "$model" ]]; then
    case "$model" in
    fable | opus | sonnet | haiku | best | opusplan | "sonnet[1m]" | "opus[1m]" | inherit | default | claude-*)
      ;;
    *)
      echo "invalid-model    $rel: '$model'"
      fm_failed=1
      ;;
    esac
  fi

  # A concrete dated model id (claude-<family>-<digits...>) always goes
  # stale: the family's default id changes on every release, so a pin here
  # either silently drifts behind the alias or, if the id retires, falls
  # back with no signal (see the 2.1.220 divergence report this doctor
  # already tracks below). Aliases always resolve to the current default;
  # require one instead of a snapshot id, in both skills and agents. No
  # opt-out: the case-list's bare `claude-*` above only keeps a non-dated
  # custom value (no digits, e.g. a hypothetical "claude-preview") from
  # tripping invalid-model - any digit-bearing form still lands here
  # unconditionally. Nothing in this repo needs to pin a dated snapshot on
  # purpose today; add a documented opt-out (e.g. a `model-pin-reason:`
  # field) only when a real use case shows up, not preemptively.
  if [[ "$model" =~ ^claude-([a-z]+-)?[0-9] ]]; then
    echo "dated-model-pin  $rel: '$model' is a concrete pin, not an alias - use opus/sonnet/haiku/fable instead"
    fm_failed=1
  fi

  if [[ "$model" == "haiku" && -n "$effort" ]]; then
    echo "extra-effort     $rel: haiku does not support effort (got '$effort')"
    fm_failed=1
  fi

  if [[ -n "$effort" ]]; then
    case "$effort" in
    low | medium | high | xhigh | max)
      ;;
    *)
      echo "invalid-effort   $rel: '$effort'"
      fm_failed=1
      ;;
    esac
  fi

  if [[ -n "$session_model" && "$model" == "$session_model" ]]; then
    echo "redundant-pin    $rel: model '$model' matches the session default in settings.json; pin nothing instead"
    fm_failed=1
  fi

  if [[ -n "$session_effort" && "$effort" == "$session_effort" ]]; then
    echo "redundant-pin    $rel: effort '$effort' matches the session default in settings.json; pin nothing instead"
    fm_failed=1
  fi

  # Claude Code 2.1.220 silently drops the model override when a skill sets both
  # model: and effort: (upstream bug, issue filed; remove this check once fixed).
  # Confirmed skills/slash-command-path only: an agent dispatched with an
  # explicit model override resolved correctly despite pairing model:+effort:
  # (re-tested 2026-08-01). Keep this check skills-scoped until the upstream
  # bug is fixed.
  if [[ "$rel" == skills/* && -n "$model" && -n "$effort" ]]; then
    echo "model-effort-pair  $rel: model+effort pairing is dropped silently by Claude Code (upstream bug, see issue); keep one"
    fm_failed=1
  fi

done < <(
  (
    find "$SKILLS_DIR" -name "SKILL.md" -type f
    find "$AGENTS_DIR" -maxdepth 1 -name "*.md" -type f
  ) | sort
)

if ((fm_failed)); then
  exit_code=1
else
  echo "ok             $fm_count skills/agents passed frontmatter lint"
fi

echo
echo "== skill map validation =="

STACKS_YML_SRC="$SOURCE_ROOT/kit.yml"
SKILLS_DIR="$SOURCE_ROOT/skills"
sm_failed=0

# Collect every skill named in skill_file_map[].skills[].
mapfile -t sfm_skills < <(yq '.skill_file_map // [] | .[].skills // [] | .[]' "$STACKS_YML_SRC" 2>/dev/null | sort -u)
for sk in "${sfm_skills[@]}"; do
  if [[ ! -d "$SKILLS_DIR/$sk" ]]; then
    echo "missing-skill    skill_file_map references '$sk' but $SKILLS_DIR/$sk/ does not exist"
    sm_failed=1
  fi
done

# Collect every key in skill_triggers.
mapfile -t trigger_skills < <(yq '.skill_triggers // {} | keys | .[]' "$STACKS_YML_SRC" 2>/dev/null | sort -u)
for sk in "${trigger_skills[@]}"; do
  if [[ ! -d "$SKILLS_DIR/$sk" ]]; then
    echo "missing-skill    skill_triggers references '$sk' but $SKILLS_DIR/$sk/ does not exist"
    sm_failed=1
  fi
done

# Every skill mapped in stacks[].skills[] or stacks[].extras[].skills[]
# (stack/extra skills; global_skills excluded) must have a skill_triggers entry.
# Plugin skills are out of scope: no mapped skill is a plugin skill.
mapfile -t stack_skills < <(
  {
    yq '.stacks | to_entries[] | .value.skills // [] | .[]' "$STACKS_YML_SRC" 2>/dev/null
    yq '.stacks | to_entries[] | .value.extras // [] | .[].skills // [] | .[]' "$STACKS_YML_SRC" 2>/dev/null
  } | sort -u
)
mapfile -t global_skill_list < <(yq '.global_skills // [] | .[]' "$STACKS_YML_SRC" 2>/dev/null | sort -u)

for sk in "${stack_skills[@]}"; do
  [[ -z "$sk" ]] && continue
  # Skip skills that are in global_skills; they are required, not suggested.
  is_global=0
  for gsk in "${global_skill_list[@]}"; do
    [[ "$gsk" == "$sk" ]] && is_global=1 && break
  done
  ((is_global)) && continue

  trigger="$(yq ".skill_triggers.\"${sk}\" // \"\"" "$STACKS_YML_SRC" 2>/dev/null || true)"
  if [[ -z "$trigger" || "$trigger" == "null" ]]; then
    echo "missing-trigger  stack/extra maps '$sk' but skill_triggers has no entry for it"
    sm_failed=1
  fi
done

if ((sm_failed)); then
  exit_code=1
else
  echo "ok             skill_file_map (${#sfm_skills[@]} skills), skill_triggers (${#trigger_skills[@]} entries), stack coverage all valid"
fi

echo
echo "== skills-log field parity =="

LOG_SKILLS="$SOURCE_ROOT/hooks/log-skills.sh"
LIB="$SOURCE_ROOT/bin/_lib.sh"
SKILLS_REPORT="$SOURCE_ROOT/bin/skills-report.sh"
field_parity_failed=0

# Field subset of skills.jsonl that skills-report.sh consumes for
# classification. bin/_lib.sh's log_event emits ts/event/session_id for
# every row; log-skills.sh passes the rest. Not the full emitted set --
# hook/cwd are emitted but never read by the report, so a rename there
# carries no drift risk worth checking.
# skills-report.sh must reference each field below verbatim in its jq
# filters, or a future rename in the emitter silently breaks the report
# instead of erroring.
skills_log_fields=(
  ts event session_id expansion_type command_name skill_file tool_name
)

# Word-boundary match, not plain substring: a bare `grep -qF` on a short name
# like "ts" also matches inside "tests"/"Reports"/"counts", which would pass
# this check even after the real field reference was renamed away.
for field in "${skills_log_fields[@]}"; do
  if ! cat "$LIB" "$LOG_SKILLS" 2>/dev/null | grep -qE "\\b${field}\\b"; then
    echo "missing-field  neither _lib.sh's log_event nor log-skills.sh emits '$field' -- update the canonical list"
    field_parity_failed=1
  fi
  if [[ -f "$SKILLS_REPORT" ]] && ! grep -qE "\\b${field}\\b" "$SKILLS_REPORT"; then
    echo "missing-field  skills-report.sh: '$field'"
    field_parity_failed=1
  fi
done

if ((field_parity_failed)); then
  exit_code=1
else
  echo "ok             ${#skills_log_fields[@]} skills.jsonl fields emitted and read by skills-report.sh"
fi

echo
echo "== audit-verify field parity =="

REPORT_FORMAT="$SOURCE_ROOT/skills/report-format/SKILL.md"
AUDIT_VERIFY="$SOURCE_ROOT/skills/audit/references/verify.md"
verify_parity_failed=0

# audit-verify's step 3 parses these per-finding fields out of a
# report-format-shaped input report. Adding a field to the report-format skill's
# required shape without updating audit-verify's parser (or vice versa)
# silently breaks the re-check.
finding_fields=(Severity Location What "Why it matters" Fix Refs)

# Word-boundary match, not plain substring: short names like "What" or "Fix"
# also match inside unrelated words, which would pass this check even after
# the real field name was renamed away.
for field in "${finding_fields[@]}"; do
  if ! grep -qE "\\b${field}\\b" "$REPORT_FORMAT" 2>/dev/null; then
    echo "missing-field  skills/report-format/SKILL.md no longer documents '$field'"
    verify_parity_failed=1
  fi
  if [[ -f "$AUDIT_VERIFY" ]] && ! grep -qE "\\b${field}\\b" "$AUDIT_VERIFY"; then
    echo "missing-field  audit/references/verify.md: '$field'"
    verify_parity_failed=1
  fi
done

if ((verify_parity_failed)); then
  exit_code=1
else
  echo "ok             ${#finding_fields[@]} per-finding fields referenced in both skills/report-format/SKILL.md and audit/references/verify.md"
fi

echo
echo "== skill directory / allow-list parity =="

SETTINGS_JSON="$PERSONAL_ROOT/settings.json"
SKILLS_DIR="$SOURCE_ROOT/skills"
skill_allow_failed=0

mapfile -t allow_skills < <(jq -r '.permissions.allow[] | select(startswith("Skill(")) | sub("^Skill\\("; "") | sub("\\)$"; "")' "$SETTINGS_JSON" 2>/dev/null | sort -u)

while IFS= read -r dir; do
  name="$(basename "$dir")"

  found=0
  for sk in "${allow_skills[@]}"; do
    [[ "$sk" == "$name" ]] && found=1 && break
  done
  if ((! found)); then
    echo "missing-allow  $name has no Skill($name) entry in settings.json allow"
    skill_allow_failed=1
  fi
done < <(find "$SKILLS_DIR" -mindepth 1 -maxdepth 1 -type d | sort)

if ((skill_allow_failed)); then
  exit_code=1
else
  echo "ok             every skills/ directory has a matching Skill(<name>) allow entry"
fi

echo
echo "== CLAUDE.md rules pointer parity =="

pointer_failed=0
while IFS= read -r ref; do
  [[ -f "$SOURCE_ROOT/$ref" || -f "$PERSONAL_ROOT/$ref" ]] || {
    echo "missing-file   CLAUDE.md points at $ref, no such file"
    pointer_failed=1
  }
done < <(grep -oE 'rules/[a-zA-Z0-9_-]+\.md' "$PERSONAL_ROOT/CLAUDE.md" | sort -u)

if ((pointer_failed)); then
  exit_code=1
else
  echo "ok             every rules/*.md reference in CLAUDE.md resolves to a real file"
fi

echo
echo "== settings.json machine-local leak =="

SETTINGS_JSON="$PERSONAL_ROOT/settings.json"
leak_failed=0

# autoMode.environment is machine-appended per-repo context (see doctor.sh's
# header comment, check 9). Scoped to just this array, not the whole file:
# one of these patterns also appears legitimately in extraKnownMarketplaces'
# public plugin-marketplace source, which is not the leak this guards against.
leak_patterns=(
  "spacelift-io/"
  "DATADOG_API_KEY"
  "DD_BUGSNAG"
  ".env.k8s_simulate"
)

env_blob="$(jq -r '.autoMode.environment // [] | .[]' "$SETTINGS_JSON" 2>/dev/null)"
for pat in "${leak_patterns[@]}"; do
  if grep -qF "$pat" <<<"$env_blob"; then
    echo "leaked-key     settings.json autoMode.environment contains '$pat'; org-internal detail does not belong in a public repo"
    leak_failed=1
  fi
done

if ((leak_failed)); then
  exit_code=1
else
  echo "ok             autoMode.environment carries no leaked org-internal detail"
fi

echo
echo "== mcp allow-list server parity =="

if ! command -v claude >/dev/null 2>&1; then
  echo "skip           claude CLI not found; skipping mcp allow-list server parity"
else
  SETTINGS_JSON="$PERSONAL_ROOT/settings.json"

  # Known servers, normalized the same way a permission rule's server segment
  # is derived from a server's `claude mcp list` display name: non-alphanumeric
  # runs collapse to a single underscore ("claude.ai Figma" -> "claude_ai_Figma",
  # "plugin:github:github" -> "plugin_github_github"). The name is everything
  # before the transport (a URL, or a bare command for local/stdio servers).
  mapfile -t known_servers < <(
    claude mcp list 2>/dev/null |
      grep -vE '^(Checking |\[|$)' |
      awk '{ if ($0 ~ /https?:\/\//) { sub(/: *https?:\/\/.*/, "") } else { sub(/:.*/, "") } print }' |
      tr -c 'A-Za-z0-9\n' '_' |
      sed -E 's/_+/_/g; s/^_//; s/_$//' |
      sort -u
  )

  if ((${#known_servers[@]} == 0)); then
    echo "skip           \`claude mcp list\` returned no servers; skipping mcp allow-list server parity"
  else
    mapfile -t mcp_allow < <(jq -r '.permissions.allow[] | select(startswith("mcp__"))' "$SETTINGS_JSON" 2>/dev/null | sort -u)

    plugin_entries=0
    absent_entries=0
    absent_servers=()

    for entry in "${mcp_allow[@]}"; do
      without_prefix="${entry#mcp__}"
      server="${without_prefix%%__*}"
      # Plugin-provided servers come from a plugin's own .mcp.json and never
      # appear in `claude mcp list`, so parity is unverifiable here.
      if [[ "$server" == plugin_* ]]; then
        plugin_entries=$((plugin_entries + 1))
        continue
      fi
      found=0
      for ks in "${known_servers[@]}"; do
        [[ "$ks" == "$server" ]] && found=1 && break
      done
      if ((! found)); then
        absent_entries=$((absent_entries + 1))
        absent_servers+=("$server")
      fi
    done

    if ((absent_entries)); then
      while read -r count server; do
        echo "info           $server: $count allow entries, no such server on this machine"
      done < <(printf '%s\n' "${absent_servers[@]}" | sort | uniq -c)
    fi

    if ((plugin_entries)); then
      echo "info           $plugin_entries plugin-provided entries unverifiable; \`claude mcp list\` omits plugin servers"
    fi

    echo "ok             $((${#mcp_allow[@]} - plugin_entries - absent_entries)) mcp__ allow entries match a configured server"
  fi
fi

echo
echo "== plugin hooks.json parity =="

# hooks/hooks.json is the plugin copy of settings.json's hooks, rewritten to
# ${CLAUDE_PLUGIN_ROOT}. Editing one without the other ships adopters a
# different hook set than the one this machine runs.
HOOKS_JSON="$SOURCE_ROOT/hooks/hooks.json"
expected_hooks="$(jq -S '{hooks: (.hooks | walk(if type=="object" and has("command") then .command |= ("\"${CLAUDE_PLUGIN_ROOT}/hooks/" + sub("^\\$HOME/\\.claude/hooks/"; "") + "\"") else . end))}' "$PERSONAL_ROOT/settings.json")"
if [[ "$expected_hooks" == "$(jq -S . "$HOOKS_JSON")" ]]; then
  echo "ok             hooks/hooks.json matches settings.json hooks"
else
  echo "drift          hooks/hooks.json differs from settings.json hooks; regenerate it"
  exit_code=1
fi

exit "$exit_code"
