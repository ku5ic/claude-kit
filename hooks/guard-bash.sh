#!/usr/bin/env bash
# PreToolUse hook: blocks genuinely destructive bash patterns that
# permission rules can't express reliably. exit 2 blocks (stderr is the
# reason shown to Claude); any other nonzero exit is a soft failure.

HOOK_NAME="guard-bash.sh"
# shellcheck source=_lib.sh
source "$(dirname "$0")/_lib.sh"
# shellcheck source=../bin/_lib.sh
source "$(dirname "$0")/../bin/_lib.sh"

read_payload
require_jq

cmd="$(extract_command)"
[[ -z "$cmd" ]] && exit 0

block() {
  echo "Blocked by ${HOOK_NAME}: $1" >&2
  echo "Command: $cmd" >&2
  exit 2
}

# Forces the interactive permission prompt for cases settings.json prefix
# patterns can't express (flagged/unflagged forms sharing one prefix) -
# unlike block(), hands the decision back to the user instead of denying.
force_ask() {
  jq -cn --arg reason "$1" '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "ask",
      permissionDecisionReason: $reason
    }
  }'
  exit 0
}

_cwd="$(printf '%s' "$payload" | jq -r '.cwd // ""' 2>/dev/null || true)"
norm="$(printf '%s' "$cmd" | tr '\t' ' ' | tr -s ' ')"

# Full-string checks: need the complete command chain, or are distinctive
# enough that quoted-argument false positives aren't realistic.

if [[ "$norm" =~ :\(\)[[:space:]]*\{ ]]; then
  block "fork bomb pattern" "fork-bomb"
fi

# Not per-segment: curl/wget and the interpreter sit on opposite sides of |.
if [[ "$norm" =~ (curl|wget)[[:space:]].*\|[[:space:]]*(sh|bash|zsh|fish|python|node|ruby|perl) ]]; then
  block "piping network content into an interpreter" "pipe-to-shell"
fi

if [[ "$norm" =~ \>[[:space:]]*/dev/(sd|nvme|disk|rdisk) ]]; then
  block "write to raw disk device" "device-write"
fi

if [[ "$norm" =~ \>+[[:space:]]*(\$HOME|\$\{HOME\}|~|$HOME)/\.(zshrc|zprofile|bashrc|bash_profile|profile)([[:space:]]|$) ]]; then
  block "direct write to a shell rc file. Use the dotfiles repo." "rc-redirect"
fi

# Quote-stripped copy: checks below must ignore chain operators/redirects
# that appear only inside quoted literals (grep '&&' or grep '2>&1' shouldn't
# look like a real chain or redirect).
_cmd_sq="$(printf '%s' "$cmd" | sed -E "s/'[^']*'//g" | sed -E 's/"[^"]*"//g')"

# Full-string because xargs and rm straddle a | boundary; the per-segment
# splitter does not split on |.
if [[ "$_cmd_sq" =~ xargs[[:space:]]+((-[^[:space:]]+[[:space:]]+)*)rm[[:space:]]+-[a-zA-Z]*[rRfF] ]]; then
  block "xargs rm with recursive or force flag" "xargs-rm"
fi

# Prints "manager:lockfile" for the first lockfile match in <dir> or its git
# toplevel, nothing if none found. Manager comes from bin/_lib.sh's
# resolve_package_manager (already sourced); this only adds the matching
# lockfile name for the block message.
_resolve_pm_for_dir() {
  local dir="${1:-.}"
  local mgr
  mgr="$(resolve_package_manager "$dir")"
  [[ -z "$mgr" ]] && return 0

  local toplevel
  toplevel="$(git -C "$dir" rev-parse --show-toplevel 2>/dev/null || true)"

  # Walks the cached package_managers table from bin/_lib.sh rather than
  # re-querying _stacks.yml with yq on every package-manager command.
  local i lf
  for ((i = 0; i < ${#STACK_PM_MANAGERS[@]}; i++)); do
    [[ "${STACK_PM_MANAGERS[$i]}" == "$mgr" ]] || continue
    lf="${STACK_PM_LOCKFILES[$i]}"
    [[ -z "$lf" || "$lf" == "null" ]] && continue
    if [[ -f "$dir/$lf" || (-n "$toplevel" && -f "$toplevel/$lf") ]]; then
      printf '%s:%s\n' "$mgr" "$lf"
      return 0
    fi
  done
}

# Returns 0 (true) when $1 names a sensitive credential/key file. Normalizes
# ~ and $HOME first. Patterns must match bin/doctor.sh's parity grep across
# guard-edit.sh, guard-bash.sh, and settings.json.
_is_sensitive_arg() {
  local arg="$1"
  arg="${arg/#\~/$HOME}"
  arg="${arg/#\$HOME/$HOME}"
  arg="${arg/#\$\{HOME\}/$HOME}"
  local base="${arg##*/}"
  case "$base" in
  .env | .env.*) return 0 ;;
  *.pem | *.key | *.p12 | *.pfx) return 0 ;;
  id_rsa | id_ed25519 | id_ecdsa) return 0 ;;
  esac
  case "$arg" in
  "$HOME/.ssh/"*) return 0 ;;
  "$HOME/.gnupg/"*) return 0 ;;
  "$HOME/Library/Keychains/"*) return 0 ;;
  "$HOME/.aws/credentials" | "$HOME/.aws/config") return 0 ;;
  "$HOME/.docker/config.json") return 0 ;;
  "$HOME/.config/gh/hosts.yml") return 0 ;;
  "$HOME/.netrc" | "$HOME/.pgpass" | "$HOME/.npmrc") return 0 ;;
  "$HOME/.pypirc") return 0 ;;
  "$HOME/.cargo/credentials") return 0 ;;
  "$HOME/.gem/credentials") return 0 ;;
  esac
  return 1
}

# Mirrors the path set in the full-string rc-file redirect guard above.
_is_rc_file() {
  local arg="$1"
  arg="${arg/#\~/$HOME}"
  arg="${arg/#\$HOME/$HOME}"
  arg="${arg/#\$\{HOME\}/$HOME}"
  case "$arg" in
  "$HOME/.zshrc" | "$HOME/.zprofile" | "$HOME/.bashrc" | "$HOME/.bash_profile" | "$HOME/.profile") return 0 ;;
  esac
  return 1
}

# Returns 0 (true) when $1 is a relative write target that would land loose in
# the repo instead of scratch (rules/tooling.md). Unresolvable
# targets - variables, subshells, quoted strings - return 1: $(scratch-dir.sh)
# is the sanctioned form and must not trip this.
_is_loose_write_target() {
  local p="$1"
  # shellcheck disable=SC2016  # matching a literal "$(" is the point here
  case "$p" in
  '' | - | \$* | *'$('* | \"* | \'*) return 1 ;;
  scratch | scratch/* | */scratch | */scratch/*) return 1 ;;
  /* | \~*) return 1 ;;
  esac
  return 0
}

# Per-segment checks: split on &&, ||, ;, newlines - not | so pipe chains
# like curl|bash stay intact for the full-string check above. Each segment
# is only checked when its leading token is a known dangerous command, so
# text mentioning command names (commit bodies, grep patterns) isn't scanned.
# sudo-prefixed commands aren't unwrapped; sudo forces its own confirmation.
_check_segment() {
  local seg="$1"
  seg="${seg#"${seg%%[![:space:]]*}"}"
  seg="${seg%"${seg##*[![:space:]]}"}"
  [[ -z "$seg" ]] && return 0

  local lead="${seg%% *}"

  case "$lead" in
  rm)
    if [[ "$seg" =~ rm[[:space:]]+(-[a-zA-Z]*[rRfF][a-zA-Z]*[[:space:]]+)+(/|/\*|~|~/|\$HOME|\$\{HOME\}|\.|\.\.)($|[[:space:]]) ]]; then
      block "rm with recursive force against root, home, or cwd" "rm-recursive"
    fi
    ;;
  dd | shred | wipefs | mkfs | mkfs.*)
    block "low level disk or filesystem tool" "disk-tool"
    ;;
  chmod)
    if [[ "$seg" =~ chmod[[:space:]]+(-R[[:space:]]+)?777([[:space:]]|$) ]]; then
      block "chmod 777" "chmod-777"
    fi
    if [[ "$seg" =~ chmod[[:space:]] ]] && [[ "$seg" =~ \+x ]]; then
      if [[ "$seg" =~ [[:space:]](\.|\.\.|/)($|[[:space:]]) ]] ||
        [[ "$seg" =~ [[:space:]](~|\$HOME|\$\{HOME\})($|[[:space:]]|/) ]]; then
        block "broad chmod +x against root, home, or cwd" "chmod-broad-x"
      fi
    fi
    ;;
  git)
    if [[ "$seg" =~ git[[:space:]]+push[[:space:]].*(--force[^-]|--force$|-f([[:space:]]|$)) ]]; then
      if [[ ! "$seg" =~ --force-with-lease ]]; then
        block "git push --force. Use --force-with-lease if you must." "git-force-push"
      fi
    fi
    if [[ "$seg" =~ git[[:space:]]+push[[:space:]].*(main|master|develop|production|release) ]]; then
      if [[ "$seg" =~ (--force[^-]|--force$|[[:space:]]-f([[:space:]]|$)) ]]; then
        block "force push to a protected branch" "git-force-push-protected"
      fi
    fi
    # Branch token bounded by whitespace/start/end: matches the actual ref,
    # not a substring in a longer name (feat/production-config, fix/mainline).
    if [[ "$seg" =~ git[[:space:]]+push[[:space:]] ]] && [[ "$seg" =~ (^|[[:space:]])(origin/)?(main|master|develop|production|release)([[:space:]]|$) ]]; then
      block "push to a protected branch; use a feature branch" "git-push-protected"
    fi
    if [[ "$seg" =~ git[[:space:]]+reset[[:space:]]+--hard[[:space:]]+(origin/)?(main|master|develop|production) ]]; then
      block "git reset --hard on protected branch" "git-reset-hard"
    fi
    if [[ "$seg" =~ git[[:space:]]+(commit|push|merge|rebase)[[:space:]].*--no-verify ]]; then
      block "use of --no-verify bypasses pre-commit and pre-push hooks" "git-no-verify"
    fi
    if [[ "$seg" =~ git[[:space:]]+config[[:space:]]+--global ]]; then
      block "git config --global from a project session" "git-config-global"
    fi
    # Tree-wide pathspecs only: bare dot, double-dash-dot, :/, or bare star.
    # --staged without --worktree is allowed (unstaging isn't destructive).
    if [[ "$seg" =~ [[:space:]](restore|checkout)[[:space:]] ]]; then
      if [[ "$seg" =~ [[:space:]](\.|\*|--[[:space:]]?\.|:/)([[:space:]]|$) ]]; then
        if ! [[ "$seg" =~ --staged ]] || [[ "$seg" =~ --worktree ]]; then
          block "tree-wide discard of working-tree changes; restore individual files explicitly" "git-tree-discard"
        fi
      fi
    fi
    ;;
  psql)
    if [[ "$seg" =~ psql[[:space:]].*(-c|--command)[[:space:]] ]]; then
      if [[ "$seg" =~ (DROP[[:space:]]+(DATABASE|SCHEMA|TABLE)|TRUNCATE[[:space:]]+TABLE|DELETE[[:space:]]+FROM[[:space:]]+[a-zA-Z_]+[[:space:]]*;|DELETE[[:space:]]+FROM[[:space:]]+[a-zA-Z_]+[[:space:]]*$) ]]; then
        block "destructive SQL via psql -c" "psql-destructive"
      fi
    fi
    ;;
  redis-cli)
    if [[ "$seg" =~ redis-cli[[:space:]].*(FLUSHALL|FLUSHDB|CONFIG[[:space:]]+SET|DEBUG[[:space:]]+SLEEP) ]]; then
      block "destructive redis-cli command" "redis-destructive"
    fi
    ;;
  aws)
    # ([^[:space:]]+[[:space:]]+)* skips global flags (--profile prod)
    # between binary and verb without a bare .*'s ambiguity, which could
    # latch onto an unrelated "s3" substring inside an argument (an s3://
    # URI) and silently defeat the check.
    if [[ "$seg" =~ aws[[:space:]]+([^[:space:]]+[[:space:]]+)*s3[[:space:]]+rm[[:space:]].*(--recursive)([[:space:]]|$) ]]; then
      block "aws s3 rm --recursive deletes an entire bucket prefix" "aws-s3-recursive-rm"
    fi
    if [[ "$seg" =~ aws[[:space:]]+([^[:space:]]+[[:space:]]+)*s3[[:space:]]+rb[[:space:]].*(--force)([[:space:]]|$) ]]; then
      block "aws s3 rb --force force-deletes a bucket and its contents" "aws-s3-force-rb"
    fi
    if [[ "$seg" =~ aws[[:space:]]+([^[:space:]]+[[:space:]]+)*ec2[[:space:]]+terminate-instances ]]; then
      block "aws ec2 terminate-instances is irreversible" "aws-ec2-terminate"
    fi
    ;;
  gcloud)
    if [[ "$seg" =~ gcloud[[:space:]]+([^[:space:]]+[[:space:]]+)*delete([[:space:]]|$) ]]; then
      block "gcloud delete operation" "gcloud-delete"
    fi
    ;;
  kubectl)
    # See the aws case above for why tokens are skipped explicitly.
    if [[ "$seg" =~ kubectl[[:space:]]+([^[:space:]]+[[:space:]]+)*delete([[:space:]]|$) ]]; then
      block "kubectl delete" "kubectl-delete"
    fi
    ;;
  terraform)
    if [[ "$seg" =~ terraform[[:space:]]+([^[:space:]]+[[:space:]]+)*destroy([[:space:]]|$) ]]; then
      block "terraform destroy" "terraform-destroy"
    fi
    if [[ "$seg" =~ terraform[[:space:]]+([^[:space:]]+[[:space:]]+)*apply[[:space:]].*(-auto-approve|--auto-approve)([[:space:]]|$) ]]; then
      block "terraform apply -auto-approve skips the plan review step" "terraform-auto-approve"
    fi
    ;;
  docker)
    if [[ "$seg" =~ (system|volume|image|container|network)[[:space:]]+prune ]]; then
      # -a/-f are the only short flags these prune subcommands define, so any
      # cluster with both letters (-af, -fa) is equivalent to --all --force.
      local _has_a=0 _has_f=0
      [[ "$seg" =~ (^|[[:space:]])(-a|--all)([[:space:]]|$) || "$seg" =~ (^|[[:space:]])-[a-zA-Z]*a[a-zA-Z]*([[:space:]]|$) ]] && _has_a=1
      [[ "$seg" =~ (^|[[:space:]])(-f|--force)([[:space:]]|$) || "$seg" =~ (^|[[:space:]])-[a-zA-Z]*f[a-zA-Z]*([[:space:]]|$) ]] && _has_f=1
      if ((_has_a && _has_f)); then
        block "docker prune with --all --force wipes all unused resources" "docker-prune-all-force"
      fi
    fi
    ;;
  find)
    if [[ "$seg" =~ find[[:space:]].*-delete($|[[:space:]]) ]]; then
      block "find -delete" "find-delete"
    fi
    if [[ "$seg" =~ find[[:space:]].*-exec[[:space:]]+rm([[:space:]]|$) ]]; then
      block "find -exec rm" "find-exec-rm"
    fi
    ;;
  security)
    if [[ "$seg" =~ security[[:space:]]+delete-keychain ]]; then
      block "keychain deletion" "keychain-delete"
    fi
    ;;
  npm | npx | pnpm | yarn | bun | bunx | pip | pip3 | poetry | uv | pipenv)
    # settings.json can't express "this prefix except with this flag", so
    # pnpm install without --frozen-lockfile (which can change the lockfile)
    # forces a prompt here instead.
    if [[ "$lead" == "pnpm" ]]; then
      local _pnpm_rest="${seg#pnpm}"
      _pnpm_rest="${_pnpm_rest#"${_pnpm_rest%%[![:space:]]*}"}"
      if [[ "$_pnpm_rest" =~ ^(install|i)([[:space:]]|$) ]] && [[ ! "$seg" =~ (^|[[:space:]])--frozen-lockfile([[:space:]]|$) ]]; then
        force_ask "pnpm install without --frozen-lockfile can change the lockfile; confirm before running"
      fi
    fi
    if [[ "$lead" =~ ^(npm|pnpm|yarn)$ ]] && [[ "$seg" =~ (npm|pnpm|yarn)[[:space:]]+(install|add|i)[[:space:]]+.*(-g|--global) ]]; then
      block "global package install. Use a project-local install or asdf shim." "pkg-global-install"
    fi
    if [[ "$lead" == "yarn" ]] && [[ "$seg" =~ yarn[[:space:]]+global[[:space:]]+add[[:space:]] ]]; then
      block "global package install. Use a project-local install or asdf shim." "pkg-global-install"
    fi
    if [[ "$lead" == "bun" ]] && [[ "$seg" =~ bun[[:space:]]+(add|install)[[:space:]]+.*(-g|--global) ]]; then
      block "global package install. Use a project-local install or asdf shim." "pkg-global-install"
    fi
    # Block when the invoked PM differs from the lockfile-detected one.
    # --version/-v is exempt (never touches project files).
    local _pm_rest
    _pm_rest="${seg#"$lead"}"
    _pm_rest="${_pm_rest#"${_pm_rest%%[![:space:]]*}"}"
    [[ "$_pm_rest" == "--version" || "$_pm_rest" == "-v" ]] && return 0
    # Canonical invoked PM (aliases: npx->npm, bunx->bun, pip3->pip).
    # Known gap: python -m pip bypasses this (lead token is python, not pip).
    local _invoked
    case "$lead" in
    npx) _invoked="npm" ;;
    bunx) _invoked="bun" ;;
    pip3) _invoked="pip" ;;
    *) _invoked="$lead" ;;
    esac
    # Greenfield (no lockfile) is always allowed.
    local _pm_info
    _pm_info="$(_resolve_pm_for_dir "${_cwd:-$PWD}")"
    [[ -z "$_pm_info" ]] && return 0
    local _expected="${_pm_info%%:*}"
    local _lf_found="${_pm_info#*:}"
    [[ "$_invoked" == "$_expected" ]] && return 0
    # npx/bunx get their dlx equivalents in the suggested replacement.
    local _tail="${seg#"$lead"}"
    local _suggest
    if [[ "$lead" == "npx" ]]; then
      case "$_expected" in
      bun) _suggest="bunx${_tail}" ;;
      pnpm) _suggest="pnpm dlx${_tail}" ;;
      yarn) _suggest="yarn dlx${_tail}" ;;
      *) _suggest="${_expected}${_tail}" ;;
      esac
    elif [[ "$lead" == "bunx" ]]; then
      case "$_expected" in
      npm) _suggest="npx${_tail}" ;;
      pnpm) _suggest="pnpm dlx${_tail}" ;;
      yarn) _suggest="yarn dlx${_tail}" ;;
      *) _suggest="${_expected}${_tail}" ;;
      esac
    else
      _suggest="${_expected}${_tail}"
    fi
    block "this repo uses ${_expected} (${_lf_found}); rerun as: ${_suggest}" "pm-mismatch"
    ;;
  curl)
    # -O/-J let the server name the file and drop it in cwd; there is no case
    # where that is intended. An explicit -o/--output-dir target still has to
    # resolve outside the repo, so a relative one gets a prompt.
    local _carg _remote=0 _want_target=0 _target=""
    for _carg in ${seg#"$lead"}; do
      if ((_want_target)); then
        _target="$_carg"
        _want_target=0
        continue
      fi
      case "$_carg" in
      --) break ;;
      --remote-name | --remote-name-all | --remote-header-name) _remote=1 ;;
      --output | --output-dir) _want_target=1 ;;
      --*) ;;
      -*)
        [[ "$_carg" == *O* || "$_carg" == *J* ]] && _remote=1
        # Only a trailing -o consumes the next word as its value.
        [[ "$_carg" == *o ]] && _want_target=1
        ;;
      esac
    done
    if ((_remote)); then
      block "curl -O/-J writes a server-named file into the current directory; use: curl -o \"\$(scratch-dir.sh)/<name>\"" "download-to-repo"
    fi
    if _is_loose_write_target "$_target"; then
      force_ask "curl would write '${_target}' into the repo; rules/tooling.md wants \$(scratch-dir.sh)/<name>. Confirm only if this file belongs in the project tree."
    fi
    ;;
  wget)
    # wget's default is to write into cwd, so an output flag is mandatory.
    # -O - is stdout; -P names a directory, -O a file. Both get path-checked.
    local _warg _want_doc=0 _want_dir=0 _doc="" _dir="" _has_out=0
    for _warg in ${seg#"$lead"}; do
      if ((_want_doc)); then
        _doc="$_warg"
        _want_doc=0
        _has_out=1
        continue
      fi
      if ((_want_dir)); then
        _dir="$_warg"
        _want_dir=0
        _has_out=1
        continue
      fi
      case "$_warg" in
      --) break ;;
      --output-document=*)
        _doc="${_warg#*=}"
        _has_out=1
        ;;
      --directory-prefix=*)
        _dir="${_warg#*=}"
        _has_out=1
        ;;
      --output-document) _want_doc=1 ;;
      --directory-prefix) _want_dir=1 ;;
      --*) ;;
      -O) _want_doc=1 ;;
      -P) _want_dir=1 ;;
      -*) ;;
      esac
    done
    if ((_has_out == 0)); then
      block "wget writes into the current directory by default; use: wget -P \"\$(scratch-dir.sh)\" <url>" "download-to-repo"
    fi
    local _wtarget="${_doc:-$_dir}"
    if _is_loose_write_target "$_wtarget"; then
      force_ask "wget would write '${_wtarget}' into the repo; rules/tooling.md wants \$(scratch-dir.sh). Confirm only if this file belongs in the project tree."
    fi
    ;;
  cat | bat | head | tail | less | more | strings)
    local _sarg
    for _sarg in ${seg#"$lead"}; do
      case "$_sarg" in
      --) break ;;
      -*) ;;
      *)
        if _is_sensitive_arg "$_sarg"; then
          block "reading a sensitive file is not permitted" "sensitive-read"
        fi
        ;;
      esac
    done
    ;;
  grep | rg)
    # The first non-option argument is the search pattern, not a path; skip it.
    local _sarg _seen_pat=0
    for _sarg in ${seg#"$lead"}; do
      case "$_sarg" in
      --) break ;;
      -*) ;;
      *)
        if ((_seen_pat == 0)); then
          _seen_pat=1
        elif _is_sensitive_arg "$_sarg"; then
          block "reading a sensitive file is not permitted" "sensitive-read"
        fi
        ;;
      esac
    done
    ;;
  sh | bash | zsh | dash)
    # -c wrapping runs an arbitrary command string that never surfaces as its
    # own Bash tool call, bypassing the permission allow list. Scan
    # short-option clusters only; long options like --login can't carry -c.
    local _iarg _interp_c=0
    for _iarg in ${seg#"$lead"}; do
      case "$_iarg" in
      --) break ;;
      --*) ;;
      -*) [[ "$_iarg" == *c* ]] && _interp_c=1 ;;
      *) break ;;
      esac
    done
    if ((_interp_c)); then
      block "interpreter -c wrapping bypasses the permission allow list; run the command directly as a Bash tool call" "interpreter-c-wrap"
    fi
    ;;
  sed)
    # Catches sed -i on rc files; the full-string guard above only catches
    # > ~/.zshrc, not sed -i.
    local _has_i=0 _sarg
    for _sarg in ${seg#"$lead"}; do
      case "$_sarg" in
      --) break ;;
      -*) [[ "$_sarg" == *i* ]] && _has_i=1 ;;
      esac
    done
    if ((_has_i)); then
      for _sarg in ${seg#"$lead"}; do
        case "$_sarg" in
        --) break ;;
        -*) ;;
        *)
          if _is_rc_file "$_sarg"; then
            block "in-place edit of a shell rc file. Use the dotfiles repo." "rc-inplace-edit"
          fi
          ;;
        esac
      done
    fi
    ;;
  sd)
    # Always in-place when given a file argument; no flag check needed.
    local _sarg
    for _sarg in ${seg#"$lead"}; do
      case "$_sarg" in
      --) break ;;
      -*) ;;
      *)
        if _is_rc_file "$_sarg"; then
          block "in-place edit of a shell rc file. Use the dotfiles repo." "rc-inplace-edit"
        fi
        ;;
      esac
    done
    ;;
  esac
}

while IFS= read -r _seg; do
  _check_segment "$_seg"
done < <(printf '%s\n' "$norm" | sed -E 's/[[:space:]]*(&&|\|\|)[[:space:]]*/\n/g' | tr ';' '\n')

# Redirected output to a bare filename or ./name lands in cwd - the repo root,
# in a project session. Narrower than _is_loose_write_target on purpose: a
# subdir target (docs/report.md) is plausibly a deliverable, a bare one is not.
# The char class drops >&2, >(...), and /dev/* before they reach the check.
_redir_re='(^|[[:space:]])[0-9]?>>?[[:space:]]*([^[:space:]&|$"'"'"'();<>]+)'
if [[ "$norm" =~ $_redir_re ]]; then
  _redir_target="${BASH_REMATCH[2]}"
  if [[ "$_redir_target" != */* || "$_redir_target" == ./* ]]; then
    if _is_loose_write_target "$_redir_target"; then
      force_ask "'> ${_redir_target}' writes into the current directory; rules/tooling.md wants > \"\$(scratch-dir.sh)/${_redir_target##*/}\". Confirm only if this file belongs in the project tree."
    fi
  fi
fi

exit 0
