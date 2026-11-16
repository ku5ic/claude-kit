---
name: audit-readiness
description: Go or no-go review before a launch, from the repo and the monitoring connector, with gaps filed as tickets and reruns until green
argument-hint: "<service, path, or plan> | <path to a prior readiness report>"
disable-model-invocation: true
allowed-tools:
  - Bash(git log *)
  - Bash(kit detect-stack)
  - Bash(kit scratch-dir *)
---

# /audit-readiness

Decide whether a service or change is ready to launch: an owner, SLOs and alerts with playbooks, a rollback path, and a plan for each dependency's failure. Each gap can become a ticket, and a rerun shows what's still open. Read-only on the repo and on the monitoring tool.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                                            | Read                                                       |
| ------------------------------------------------- | ---------------------------------------------------------- |
| Checking ownership                                | [reference/ownership.md](reference/ownership.md)           |
| Checking SLOs, alerts, playbooks, and error paths | [reference/slo-and-alerts.md](reference/slo-and-alerts.md) |
| Checking the rollback path and flags              | [reference/rollback.md](reference/rollback.md)             |
| Checking dependencies                             | [reference/dependencies.md](reference/dependencies.md)     |

Timeouts, retries, and breakers are `/audit-arch`'s Failure isolation check, and log and metric code the stack's pattern skill's. This audit links their findings and doesn't repeat them.

## Procedure

1. Inputs. The launch scope from the arguments, a service, a path, or a plan. None, ask. A prior readiness report's path means a rerun.
2. The monitoring connector, per `skills/kit-rules/reference/tooling.md` section 4. None, say so, and mark the SLO and alert checks "repo only".
3. Read the repo for owners, flags, the rollback path, alert rules, playbook links, and dependency calls, and the connector for alerts, SLOs, and dashboards. Never write to either.
4. Check, in order, ownership, SLOs and alerts, rollback, then dependencies, each by its reference. A check neither source can answer is `unverifiable`, never a pass.
5. The verdict. No owner, no rollback path, or a paging alert with no playbook is `no-go`. A gap with a stated mitigation ranks lower but stays a gap.

## Rerun

With a prior report, each of its gaps is reclassified the way `/audit-verify` classifies findings (resolved, unresolved, moved, regressed, unverifiable). Steps 2 to 4 run again, and a gap the old report didn't have goes under "New since last run", since a launch gate that ignores new gaps isn't one. The old report is never edited.

## Report

Load the report-format skill and use its format, written to the path `kit scratch-dir readiness <scope-slug>` prints, with these narrowings:

- The TL;DR's first bullet is the verdict in place of the health word: `go`, `go with: <gaps>`, or `no-go: <gap>`.
- A Dependencies table, one row per dependency, with its failure modes and what the launch does in each.
- Each finding names its area, its evidence (`file:line`, or the connector's field and value), and its source.
- The PRR's capacity and performance areas aren't covered; the Scope line says so.

## Tickets

Ask with AskUserQuestion whether to file the gaps as tickets. On yes, run the write-tickets skill's procedure with this report as its source, one ticket per gap grouped by area in severity order, through its own post step and its yes.

## Reply

The report path, the verdict, and one next action.

Arguments: `$ARGUMENTS`
