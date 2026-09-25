#!/usr/bin/env bash
# PreToolUse hook: inspects git commit commands for AI signatures and,
# per rules/output.md section 1, an unchunked wall of text in the body.
HOOK_NAME="guard-commit.sh"
# shellcheck source=_lib.sh
source "$(dirname "$0")/_lib.sh"

read_payload
require_jq

cmd="$(extract_command)"
[[ "$cmd" =~ git[[:space:]]+([^[:space:]]+[[:space:]]+)*commit ]] || exit 0

# Line-by-line grep finds the trailer on its own line; avoids -z (GNU-only).
if printf '%s' "$cmd" | grep -qiE 'Co-Authored-By:[[:space:]]*Claude|Generated[[:space:]]+(by|with)[[:space:]]+Claude|🤖[[:space:]]*Generated'; then
  block "AI signature in commit message" "ai-commit-sig"
fi

# gitleaks exit codes: 0 clean, 1 leak found, anything else is an operational
# error that must fail open, not be conflated with a real finding. Runs
# before the subject-parsing early-exit: unlike the AI-tell check, secret
# scanning must not depend on whether -m used a quoted message.
if command -v gitleaks >/dev/null 2>&1; then
  _cwd="$(printf '%s' "$payload" | jq -r '.cwd // empty' 2>/dev/null || true)"
  _gl_status=0
  gitleaks git --staged --no-banner --redact --log-level error "${_cwd:-.}" >/dev/null 2>&1 || _gl_status=$?
  if [[ "$_gl_status" -eq 1 ]]; then
    block "gitleaks flagged a secret in the staged diff" "staged-secret"
  elif [[ "$_gl_status" -ne 0 ]]; then
    echo "${HOOK_NAME:-hook}: gitleaks exited $_gl_status (not a leak signal); skipping scan" >&2
  fi
fi

# This repo's commit convention passes multi-line messages via a heredoc
# (`-m "$(cat <<'EOF' ... EOF)"`); pull the body between opener and closing
# EOF. A plain single-line `-m "..."` has no heredoc and is skipped - short
# enough that a miss is harmless.
heredoc_body="$(printf '%s\n' "$cmd" | sed -n "/<<-\\{0,1\\}['\"]\\{0,1\\}EOF['\"]\\{0,1\\}/,/^EOF\$/p" | sed '1d;$d' || true)"
if [[ -n "$heredoc_body" ]]; then
  run="$(longest_prose_run "$heredoc_body")"
  if ((run > 4)); then
    block "commit message has an unchunked wall of text (${run} consecutive prose lines). rules/output.md section 1: short paragraphs, no dense blocks." "commit-wall-of-text"
  fi
fi

# Subject only (first line of -m value); single-line grep is sufficient.
msg_dq="$(printf '%s' "$cmd" | grep -oE '(-m|--message=?)[[:space:]]*"[^"]*"' | sed -E 's/.*"([^"]*)"/\1/' || true)"
msg_sq="$(printf '%s' "$cmd" | grep -oE "(-m|--message=?)[[:space:]]*'[^']*'" | sed -E "s/.*'([^']*)'/\1/" || true)"
subject="$(printf '%s' "${msg_dq}${msg_sq}" | head -1)"

[[ -z "$subject" ]] && exit 0

if echo "$subject" | grep -qiE '^(feat|fix|chore|refactor|docs|test|perf|build|ci|style)?:?[[:space:]]*(certainly|here is|i have|let me|in this commit|this commit)'; then
  block "AI-tell phrasing in commit subject" "ai-commit-tell"
fi

exit 0
