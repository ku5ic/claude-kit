# 13. Release 0.13: kit rules outrank harness defaults, and the guards' holes closed

Date: 2026-11-20

## Status

Accepted

Amends [12. Release 0.12.1: mods for what the kit draws](0012-release-0-12-1-mods-for-what-the-kit-draws.md)

## Context

Sources are the commits on `feat/release-0-13-0-kit-rules-outrank-harness-defaults`, each named where it's used, and the files they change.

- Before `dba9678`, nothing in the system prompt put the kit's rules ahead of a mode's or the harness's own instructions, and before `6ec8945` the kit's hook context carried no label telling it from the harness's.
- Before the fixes below, a credential read passed through a compound command's redirect (`8335f86`), NotebookEdit (`cc0cc54`), an MCP tool's path (`eda6be1`), or a Bash write target (`d00cfb7`), and a report or post written through Bash skipped the claims gate (`cee3aac`).
- ADR 9's Config decision says new behavior gets no new block in `kit.yml`. `attribution:` and `architecture_dir:` arrived after it, in `ca78f28`, with no ADR.

## Decision

- A mod, `mods/precedence`, appends a `kit:precedence` system-prompt section: the kit's rules win over default, mode, and built-in skill instructions where they conflict (`dba9678`). This amends ADR 12, which covered only what the kit draws. The section has its own `precedence` switch, and its reply-length and step-pause clauses stay in the rules files for when it's off.
- Every hook context opens with `kit:`, and the nudges state facts (`6ec8945`, `857f7c8`, `30a802f`). Blocks keep their `kit <area>: BLOCKED` headline.
- A second mod adds `/kit-stats`, behind its own `kit_stats` switch (`ac283e9`).
- A plan line `<!-- critic: skip <reason> -->` skips plan-critic, logged and counted, and the plan's claims are still checked (`d71c861`).
- New blocks: the credential routes above, more credential paths (`649f904`), `git -c core.hooksPath`, an AI tool as `--author`, and what `git rebase -x` runs (`055656f`), and a scratch report a Bash command wrote with unchecked claims (`cee3aac`).
- New asks: history git can't bring back and the whole project sent to another host (`055656f`), a deleted test file (`6e78b57`), and an edit that changes a task the gates run (`19529aa`).
- The claims gate checks `/deps`, `/pr-review`, and plan-critic reports, terse claims and headings, and what `gh` and `glab` post (`cee3aac`). It holds a text that still states a flagged claim (`cc25c5d`), passes only the first rewrite after a run (`470cbfd`), and rechecks a revised subagent report (`bc4c1e3`).
- The rules gate denies on a missing skills log and counts a skill read only in a skills dir, and a gate skipped with code left unchecked runs again at Stop (`d3176c4`). A done plan's Stop holds until a review launches (`cf04879`).
- A session's first fail-open is shown to the user (`5ab281b`).
- `claudecli` owns the failed-run cooldown, and a failed verifier run pauses the claim check for an hour (`a3984a2`). The verifier is denied every `sensitive_paths` read (`6125b9c`).
- Three holes stay open and are listed in `SECURITY.md`'s known gaps (`216f792`): one plan-critic run covers later versions of the plan, a background agent's edit after the turn counts as the user's, and Bash writes are checked after they land.
- Of ADR 9's Config decision, this holds: `kit.yml` stays small, and data another file holds is derived from it. A new key needs the author's yes, as `CLAUDE.md` says, which `attribution:`, `architecture_dir:`, and `connectors.path_fields` (`eda6be1`) had.
- Deferred: three readers of a project's declared dependencies, `sources/deps.go`, `codegraph/manifests.go`, and `pkgcheck/imports.go`, stay apart.

## Consequences

- `bash-dispatch` and `bash-edits` run under 300 s timeouts, so a verifier run fits (`cee3aac`). Git calls end a second before the hook's own timeout (`cf04879`).
- During an outage every claim check is skipped for the hour, each named to the user as `NOT CHECKED` (`a3984a2`).
- A plan whose transcript can't be read goes through with a `NOT CHECKED` notice (`d3176c4`).
- A changed Rake task body raises no ask, since Rake tasks are read by name only (`go/internal/sources/providers.go`).
