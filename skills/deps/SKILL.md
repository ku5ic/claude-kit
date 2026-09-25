---
description: Merge open Dependabot PRs and reconcile GitHub security alerts
argument-hint: "<optional: ecosystem filter, PR number, or --fix-transitive>"
disable-model-invocation: true
---

General core, best-effort tail. Package-manager agnostic.

The core (Phases 1-4) is ecosystem-agnostic: GitHub normalizes Dependabot PRs and security alerts identically across ecosystems, so merging PRs and reconciling alerts works the same everywhere. The tail (Phase 5) is opt-in.

This command names no specific package manager, lockfile, or manifest.

- Package manager and stack are already injected at session start (`<tooling>` block: `package-manager: <pm>`; `<repo-context>` block: stack and location). Read them from there. For any install, audit, tree query, lockfile regen, or override, derive the correct command from that manager at runtime.
- Do not detect the stack.
- Do not re-list lockfiles.
- Do not hardcode any tool's syntax.
- All check-running goes through `run-checks.sh`.

## Preconditions

1. Stack and package manager are in the injected `<repo-context>` and `<tooling>` blocks. Scratch directory: `!`scratch-dir.sh``.
2. Require `gh`. If absent: stop and report. Confirm auth: `gh auth status`. If unauthenticated, stop.
3. Resolve the repo slug: `gh repo view --json nameWithOwner -q .nameWithOwner`. Call it `<slug>`. If this fails there is no GitHub remote; stop, this command is GitHub-only.
4. Base branch: `!`git-base.sh``. Merge target and rebase base; do not re-derive it.
5. Working tree must be clean. If `git status --porcelain` is non-empty, stop and surface.

## Phase 1: inventory

### 1a. Dependabot PRs

`gh pr list --author "app/dependabot" --state open --json number,title,headRefName,mergeable,mergeStateStatus,labels`.

Classify each PR by semver bump from its title (`Bump x from 1.2.3 to 1.2.4`): patch, minor, major.

### 1b. Dependabot alerts (authoritative for severity; audit is only a cross-check)

Fetch all alerts with one bare GET and filter `state` client-side in jq.

- Do not pass `-f state=open`: `-f` sends a body field and makes gh issue a POST, which this read-only endpoint rejects with 404.
- Do not use a bare `?state=open` either; an unquoted `?` is a zsh glob.
- Let gh follow the Link header with `--paginate`; never construct `page`/`first`/`last`, removed for these endpoints on 2025-10-14.

```
gh api "repos/<slug>/dependabot/alerts" --paginate \
  --jq '.[] | select(.state=="open") | {number, severity: .security_vulnerability.severity, cvss: .security_advisory.cvss.score, ghsa: .security_advisory.ghsa_id, pkg: .dependency.package.name, ecosystem: .dependency.package.ecosystem, scope: .dependency.scope, relationship: .dependency.relationship, manifest: .dependency.manifest_path, fixed: .security_vulnerability.first_patched_version.identifier, range: .security_vulnerability.vulnerable_version_range}'
```

Field paths were verified against a live payload on 2026-07-02; re-check them if the endpoint's shape has moved since. The `ecosystem` field identifies each alert's ecosystem; use it. `state` is one of `open`, `fixed`, `dismissed`, `auto_dismissed`; only `open` is actionable.

Branch on the outcome (read it with `--include` and the `HTTP/2 <code>` line):

- 200: alerts are authoritative for severity. Continue.
- 404 on this bare GET:
  1. Alerts are unavailable for the repo (disabled, or not visible to the token).
  2. Record "GitHub Dependabot alerts unavailable for <slug>" (name the slug).
  3. Fall through to audit-only, flagging every severity "from local audit, may understate".
  4. Do not assert "disabled" as fact.
  5. If a 404 appears only when a filter is attached but not on the bare GET, it is a malformed request - re-issue the bare GET.
- 403: token lacks scope. A classic PAT needs `repo` or `security_events`. Stop and tell the user to run `gh auth refresh -s security_events`.
- Any other non-2xx: stop and surface status and body.

### 1c. Native audit (cross-check only, optional)

A cross-check that may surface advisories GitHub has not yet alerted on. Using the injected package manager, run that ecosystem's native audit if one exists. If the manager has no native audit or its audit tool is not installed, note that and rely on the GitHub alerts alone. A missing audit tool is not an error.

If $ARGUMENTS scopes to an ecosystem or PR number, filter to it.

## Phase 2: triage and present

Stop for approval before mutating (`rules/workflow.md` section 3 pause discipline).

- Severity is from 1b when available; where audit and alerts disagree, trust the alert.
- Map each open alert to a PR by package and fixed version.
- Annotate `relationship` and `scope`; runtime + direct + critical/high is the priority tier.

Group PRs:

- Auto-merge candidates: patch/minor, `mergeable: MERGEABLE`, `mergeStateStatus: CLEAN`.
- Conflicted: `mergeStateStatus: DIRTY` or `mergeable: CONFLICTING`.
- Hold for confirmation: major bumps, and any runtime-scope bump whose advisory flags a breaking change.

Flag open alerts with no PR. By default these are reported, not fixed (see Phase 5).

Present a table: severity, package, ecosystem, scope, relationship, current -> fixed, bump, source (alert / PR / both), action. Wait for approval.

## Phase 3: resolve conflicts

1. `gh pr comment <n> --body "@dependabot rebase"`. Poll `mergeStateStatus`.
2. If still conflicting, resolve locally:
   - `gh pr checkout <n>`
   - `git rebase <base>` (base from Precondition 4)
   - Lockfile conflict: take the incoming dependency change, then regenerate the lockfile with the injected manager's lockfile-only install. Never hand-merge a lockfile.
   - Manifest conflict: merge both edits keeping the higher compatible version, then regenerate the lockfile.
   - `git add` resolved files, `git rebase --continue`.
3. Dependabot branches are bot-owned. Confirm before `git push --force-with-lease`. Never force push a branch you do not own.

## Phase 4: verify then merge

Never merge on GitHub mergeability alone. Verify each candidate first.

Per candidate, in turn:

1. `gh pr checkout <n>`.
2. Reinstall against the PR's lockfile using the injected manager's reproducible (frozen/locked) install mode.
3. Run `!`run-checks.sh``. On a non-zero failed count: do not merge, record the failing label, leave the PR open, move on. `run-checks.sh` owns runner detection across every stack; do not reimplement it.
4. On pass: merge with the project's convention (read recent merges). Default `gh pr merge <n> --squash --delete-branch`.
5. After each merge, return to base and pull before the next PR. Re-check mergeability; a merge can newly conflict a sibling.

Process priority-tier alerts first.

## Phase 5: manual remediation of alerts with no PR (opt-in, best-effort)

Skipped unless $ARGUMENTS contains `--fix-transitive`. Without it, alerts with no Dependabot PR are reported in Phase 6 and left for the user; manual transitive remediation is ecosystem-specific and easy to get wrong unattended.

When invoked, for each open alert with no PR, work in this order and stop at the first that applies:

1. Fixed version from `security_vulnerability.first_patched_version.identifier`. If null, there is no patched release; record "no fix available" and leave open.
2. Prefer the real fix: bump the parent. Using the injected manager's dependency-tree query, find the direct dependency that pulls the vulnerable transitive in. If a newer version of that direct dependency depends on a fixed version of the transitive, bump the direct dependency. This resolves the alert through normal resolution and leaves no standing pin.
3. Only if no parent bump resolves it (parent abandoned, or its latest still pulls the vulnerable range):
   1. Determine the transitive-pin mechanism from the injected manager itself.
   2. If you are not certain the ecosystem supports a transitive pin, stop and hand back to the user with the alert details rather than guessing.
   3. Apply the transitive-pin mechanism for the injected manager.
   4. Note: a transitive pin is debt - it persists even after the ecosystem moves on.
   5. Record every pin applied.
4. If neither applies and the advisory is not reachable in this project's usage, record it for dismissal rather than forcing a change.

After any manifest edit:

1. Confirm before writing.
2. Regenerate the lockfile with the manager's lockfile-only install.
3. Verify with the manager's tree query that the tree resolved to the fixed version.
4. Run `!`run-checks.sh``.
5. Branch.
6. Commit.
7. `gh pr create`.
8. Do not push to a protected branch; stop before merging your own PR.

## Phase 6: report

Use the `rules/markdown-report.md` format. Write to `$(scratch-dir.sh)/deps-<YYYYMMDD-HHMM>.md`. Print the path.

Per PR/alert, report these fields:

| Field         | Value                                                                                                        |
| ------------- | ------------------------------------------------------------------------------------------------------------ |
| package       | package name                                                                                                 |
| ecosystem     | dependency ecosystem                                                                                         |
| scope         | `dependency.scope`                                                                                           |
| relationship  | `dependency.relationship`                                                                                    |
| version delta | current -> fixed                                                                                             |
| bump          | patch / minor / major                                                                                        |
| severity      | from the alert (or local audit if alerts were unavailable)                                                   |
| action        | merged / held / failed checks / parent bumped / pinned / manual PR opened / no fix available / left for user |

- If alerts were unavailable, state the severity source and name the slug.
- End with "Still open, needs you", listing:
  - every major bump held
  - every PR that failed checks
  - every open alert with no PR
  - if Phase 5 ran, every transitive pin applied (so the debt is visible)

## Rules

- Core (Phases 1-4) is ecosystem-agnostic. Read the `ecosystem` field on each alert; never assume a default ecosystem.
- Phase 5 is opt-in (`--fix-transitive`) and best-effort. Without the flag, report alerts with no PR and stop.
- In Phase 5, prefer a parent bump over a transitive pin. A pin is the fallback, not the default, and it is standing debt; record every one. If the ecosystem's pin mechanism is uncertain, stop and hand back rather than guess.
- Do not re-derive the base branch. Use `git-base.sh` (Precondition 4). All checks go through `run-checks.sh`.
- Severity is from the GitHub alert list when available, not local audit.
- Only `state == "open"` alerts are actionable. `fixed`, `dismissed`, `auto_dismissed` are not.
- Do not push to or merge into a protected branch directly.
- No opportunistic bumps. Touch only deps named in an open alert or open Dependabot PR.
- One mutating operation per Bash call.

## Stop conditions

Stop and hand back to the user, without proceeding further, when:

- `gh` is absent or unauthenticated (Precondition 2).
- No GitHub remote resolves (Precondition 3).
- The working tree is dirty at start (Precondition 5).
- The alerts endpoint returns 403 (Phase 1b) - token lacks scope.
- Phase 2's proposal table is presented - wait for approval before any mutation.
- A transitive-pin mechanism can't be confirmed for the ecosystem (Phase 5).
- A merge would land on or push to a protected branch (Rules).

A failed-checks PR (Phase 4) is not a full stop - record it as held and continue to the next candidate.

## Output

Report: `$(scratch-dir.sh)/deps-<YYYYMMDD-HHMM>.md` in `rules/markdown-report.md` format, per-PR/alert fields defined in Phase 6, ending with a "Still open, needs you" list.
