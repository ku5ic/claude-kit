---
name: write-runbook
description: A runbook for an alert or a service, every step checked against the repo
argument-hint: "<alert name or service>"
disable-model-invocation: true
allowed-tools:
  - Bash(git log *)
  - Bash(kit scratch-dir *)
---

# /write-runbook

Write the runbook an on-call engineer opens when an alert fires: what it means, how bad it is, how to triage, how to mitigate, and who to escalate to. Every command, flag, and link in it is checked.

Tone and shape: `skills/kit-rules/reference/output.md` section 4, which wins over this file.

<!-- chat: only
In claude.ai chat, the alert rule and what the user pasted are the material. Print the runbook in the reply, and mark each step that couldn't be checked.
-->

Loading this skill isn't enough. Before the step a row names, read its file.

| Before               | Read                                                           |
| -------------------- | -------------------------------------------------------------- |
| Drafting the runbook | [reference/runbook-practice.md](reference/runbook-practice.md) |

## Procedure

<!-- chat: skip -->

1. The alert rules, in the repo (alert files, dashboards as code) and through the monitoring connector (`skills/kit-rules/reference/tooling.md` section 4). The alert's expression, threshold, and annotations, read.
2. An existing runbook for it, by the project's own link key or runbook folder. One exists, update it instead of adding a second.
3. The service's deploy, flag, and rollback paths, read from the repo.

<!-- chat: end -->

4. Draft per [reference/runbook-practice.md](reference/runbook-practice.md).
5. Each command, flag, and link checked against the repo or the connector. One that can't be is marked unverified.

<!-- chat: skip -->

## Output file

The project's runbook folder when it has one, else the path `kit scratch-dir runbook <alert-slug>` prints. Linking it from the alert rule is a config change, made only on the user's yes.

Reply with the path and one line per unverified step.

Arguments: `$ARGUMENTS`

<!-- chat: end -->
