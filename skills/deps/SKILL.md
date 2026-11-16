---
name: deps
description: Merge open Dependabot or Renovate PRs and reconcile the code host's security alerts
argument-hint: "<optional: ecosystem filter, PR number, or --fix-transitive>"
disable-model-invocation: true
allowed-tools:
  - Bash(git status *)
  - Bash(gh auth status *)
  - Bash(gh repo view *)
  - Bash(gh pr list *)
  - Bash(glab auth status *)
  - Bash(glab repo view *)
  - Bash(glab mr list *)
---

General core, best-effort tail. Package-manager agnostic.

The core (Phases 1-4) is ecosystem-agnostic: the code host reports bot PRs and security alerts the same way for every ecosystem. The tail (Phase 5) is opt-in.

This command names no specific package manager, lockfile, manifest, or code host. Every host call is an operation in the code-host-patterns skill's Dependencies section, for the host `skills/kit-rules/reference/tooling.md` section 4 finds.

- Package manager and stack are already injected at session start (`<repo-context>` block: stack and location, and a `package-manager` line per lockfile directory). Read them from there. For any install, audit, tree query, lockfile regen, or override, derive the correct command from that manager at runtime.
- Do not detect the stack.
- Do not re-list lockfiles.
- Do not hardcode any tool's syntax.
- Never run checks locally. The gate: `skills/kit-rules/reference/verify.md` section 1.

## Preconditions

1. Find the host and its CLI (`skills/kit-rules/reference/tooling.md` section 4), load code-host-patterns, and read the host's file. No CLI and no connector, or the CLI isn't authenticated: stop and report.
2. Resolve `<repo>` with the host's repository-path call. If it fails, there's no remote on a supported host, so stop.
3. Find the bots from the repo's config files:

   | Bot        | Config file                                                                                                                                                | Branch prefix                              | Rebase request                                         |
   | ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------ | ------------------------------------------------------ |
   | Dependabot | `.github/dependabot.yml` or `.yaml`                                                                                                                        | `dependabot/`                              | the comment `@dependabot rebase`                       |
   | Renovate   | `renovate.json`, `.json5`, or `.jsonc`, the same under `.github/` or `.gitlab/`, `.renovaterc`, `.renovaterc.json*`, or a `renovate` key in `package.json` | its `branchPrefix`, by default `renovate/` | the label its `rebaseLabel` names, by default `rebase` |

   Neither: say so, and run on the security alerts alone.

4. Base branch: `kit git-base`. Merge target and rebase base; do not re-derive it.
5. Working tree must be clean. If `git status --porcelain` is non-empty, stop and surface.

## Phase 1: inventory

### 1a. Bot PRs

The host's open bot PRs, kept when the head branch starts with a bot's prefix.

Classify each PR by semver bump, patch, minor, or major: from the title when it names both versions (`Bump x from 1.2.3 to 1.2.4`), else from the version table in its body.

### 1b. Security alerts (authoritative for severity; audit is only a cross-check)

The host's security alerts, with their package, ecosystem, severity, and fixed version. Only open alerts are actionable. The host file says how each failure reads:

- Unavailable (off, the plan's tier, or hidden from the token): record "security alerts unavailable for `<repo>`", and fall through to audit-only, flagging every severity "from local audit, may understate". Never assert "disabled".
- The token lacks scope: stop, and give the host file's fix.
- Any other non-2xx: stop and surface status and body.

### 1c. Native audit (cross-check only, optional)

A cross-check that may surface advisories the host has not yet alerted on. Using the injected package manager, run that ecosystem's native audit if one exists. If the manager has no native audit or its audit tool is not installed, note that and rely on the host's alerts alone. A missing audit tool is not an error.

If $ARGUMENTS scopes to an ecosystem or PR number, filter to it.

## Phase 2: triage and present

Stop for approval before mutating (`skills/kit-rules/reference/workflow.md` sections 2 and 4).

- Severity is from 1b when available; where audit and alerts disagree, trust the alert.
- Map each open alert to a PR by package and fixed version.
- Annotate scope and relationship where the host gives them; runtime + direct + critical/high is the priority tier.

Group PRs:

- Auto-merge candidates: patch/minor, mergeable per the host file.
- Conflicted: conflicted per the host file.
- Hold for confirmation: major bumps, and any runtime-scope bump whose advisory flags a breaking change.

Flag open alerts with no PR. By default these are reported, not fixed (see Phase 5).

Present a table: severity, package, ecosystem, scope, relationship, current -> fixed, bump, source (alert / PR / both), action. Wait for approval.

## Phase 3: resolve conflicts

1. Ask the bot to rebase, its request from the Preconditions table, then poll mergeability.
2. If still conflicting, resolve locally:
   - Check the PR out with the host's checkout (`skills/kit-rules/reference/workflow.md` section 1).
   - `git rebase <base>` (base from Precondition 4)
   - Lockfile conflict: take the incoming dependency change, then regenerate the lockfile with the injected manager's lockfile-only install. Never hand-merge a lockfile.
   - Manifest conflict: merge both edits keeping the higher compatible version, then regenerate the lockfile.
   - `git add` resolved files, `git rebase --continue`.
3. Bot branches are bot-owned; force pushing one is `skills/kit-rules/reference/workflow.md` sections 1 and 2. A commit pushed to a Renovate branch stops Renovate updating it, so prefer the rebase request.

## Phase 4: verify then merge

Never merge on the host's mergeability alone (`skills/kit-rules/reference/verify.md` section 1).

Per candidate, in turn:

1. Wait for the PR's CI with the host's call. On any failing or missing check: do not merge, record the failing check, leave the PR open, move on.
2. On pass: merge with the project's convention (read recent merges), by default the host's merge, pinned to the head SHA CI ran on.
3. After each merge, re-check the next PR's mergeability; a merge can newly conflict a sibling.

Process priority-tier alerts first.

## Phase 5: manual remediation of alerts with no PR (opt-in, best-effort)

Runs only when $ARGUMENTS contains `--fix-transitive`; then read [reference/fix-transitive.md](reference/fix-transitive.md) and follow it. Without the flag, skip to Phase 6.

## Phase 6: report

Load the report-format skill and use its format. Write to the path `kit scratch-dir deps` prints.

Per PR/alert, report these fields:

| Field         | Value                                                                                                        |
| ------------- | ------------------------------------------------------------------------------------------------------------ |
| package       | package name                                                                                                 |
| ecosystem     | dependency ecosystem                                                                                         |
| scope         | runtime or development, when the host gives it                                                               |
| relationship  | direct or transitive, when the host gives it                                                                 |
| version delta | current -> fixed                                                                                             |
| bump          | patch / minor / major                                                                                        |
| severity      | from the alert (or local audit if alerts were unavailable)                                                   |
| action        | merged / held / failed checks / parent bumped / pinned / manual PR opened / no fix available / left for user |

- If alerts were unavailable, state the severity source and name `<repo>`.
- End with "Still open, needs you", listing:
  - every major bump held
  - every PR that failed checks
  - every open alert with no PR
  - if Phase 5 ran, every transitive pin applied (so the debt is visible)

## Rules

- Core (Phases 1-4) is ecosystem-agnostic. Read the `ecosystem` field on each alert; never assume a default ecosystem.
- In Phase 5, prefer a parent bump over a transitive pin. A pin is the fallback, not the default, and it is standing debt; record every one. If the ecosystem's pin mechanism is uncertain, stop and hand back rather than guess.
- Severity is from the host's alerts when available, not local audit.
- Only open alerts are actionable.
- No opportunistic bumps. Touch only deps named in an open alert or open bot PR.
- Pushing, protected branches, and mutating Bash calls: `skills/kit-rules/reference/workflow.md` sections 1 and 2.

## Stop conditions

Stop and hand back to the user, without proceeding further, when:

- No CLI or connector for the host, or it isn't authenticated (Precondition 1).
- No remote on a supported host resolves (Precondition 2).
- The working tree is dirty at start (Precondition 5).
- The alerts call says the token lacks scope (Phase 1b).
- Phase 2's proposal table is presented. Wait for approval before any mutation.
- A transitive-pin mechanism can't be confirmed for the ecosystem (Phase 5).
- A merge would land on or push to a protected branch (Rules).

A failed-checks PR (Phase 4) is not a full stop. Record it as held and continue to the next candidate.
