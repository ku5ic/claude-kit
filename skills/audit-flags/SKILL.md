---
name: audit-flags
description: Feature flag audit, stale flags, flags code and the flag service disagree on, missing owners or expiry, and the dead branches behind them
argument-hint: "<path, or nothing for the repo>"
allowed-tools:
  - Bash(git log *)
  - Bash(kit detect-stack)
  - Bash(kit scratch-dir *)
---

# /audit-flags

Audit a codebase's feature flags against its flag service: which are stale, which the code and the service disagree on, which have no owner or expiry, and what code each removal would delete. Read-only on the repo and the service.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                        | Read                                                     |
| ----------------------------- | -------------------------------------------------------- |
| Finding the flag references   | [reference/sdk-detection.md](reference/sdk-detection.md) |
| Judging staleness and removal | [reference/flag-practice.md](reference/flag-practice.md) |

## Procedure

1. Scope, the argument's path or the repo. The flag SDK comes from the manifests `kit detect-stack` names, else the project's own flag helper found with `rg`. None, say so and stop.
2. The flag service's connector, per `skills/kit-rules/reference/tooling.md` section 4. None, the audit is code-only, and the report says which checks that skipped.
3. Each code reference per [reference/sdk-detection.md](reference/sdk-detection.md), with its key, `file:line`, default, and type. Keys built at runtime go in their own list.
4. The service's flags, compared with the code both ways, with each flag's kind, age, owner, expiry, and the variations it serves.
5. Each flag classed by kind and judged against that kind's lifetime per [reference/flag-practice.md](reference/flag-practice.md).
6. For each stale or always-on flag, each reference read, naming the dead branch, its `file:line`, the literal the condition becomes, and the tests and overrides that go with it.

## Report

Load the report-format skill and use its format, written to the path `kit scratch-dir flags <scope-slug>` prints. Findings sort by severity, then by how far past its lifetime a flag is, then by references touched. Each cites the service's field and value or the `file:line`.

## Reply

The report path and the count per finding class.

## Stop

Read-only. No flag changed in the service, no code edited, no cleanup tool run.

Arguments: `$ARGUMENTS`
