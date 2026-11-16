---
name: audit-ci
description: Hardening review of CI workflow files
argument-hint: "<workflow file, or none for every file in .github/workflows>"
disable-model-invocation: true
allowed-tools:
  - Bash(git log *)
---

# /audit-ci

Hardening review of CI workflows.

## Run

Resolve a linked ticket or doc as `skills/investigate` step 0 describes, then dispatch the auditor agent (subagent_type: kit:auditor, foreground) with this file's procedure and the resolved arguments. It writes the report; reply per `skills/kit-rules/reference/output.md` section 3.

## Prerequisites

- No `.github/workflows` directory: stop and say so.

## Procedure

1. Run `actionlint` on the target workflows. Each error is a finding. With no `actionlint`, skip this step and say so under "Cannot be verified statically".
2. Read each workflow and check:
   - **Pinning**: every `uses:` of an action outside the repo is pinned to a full commit SHA, with the tag in a comment. A tag or branch is a finding; a first-party `actions/*` tag is a `warning`, a third-party one a `failure`.
   - **Permissions**: a top-level or per-job `permissions:` block granting only what the jobs use. None at all means the token's default scope, a `warning`.
   - **Untrusted triggers**: `pull_request_target` or `workflow_run` that checks out or runs the PR's code is a `failure`.
   - **Injection**: `${{ github.event.* }}`, `${{ github.head_ref }}`, or another attacker-set value interpolated into `run:` is a `failure`. The fix passes it through `env:`.
   - **Secrets**: a secret echoed, written to a file or artifact, or passed to a step that runs PR code.
   - **Self-hosted runners** on a workflow that public PRs can trigger.
3. Infra files the jobs deploy are `/audit-infra`'s job.

## Not a finding

- An event value interpolated into `run:` that an attacker can't shape: a SHA, a number, or a boolean, such as `github.event.pull_request.base.sha`. Passing it through `env:` is still cleaner, as `info`.
- A local action (`uses: ./...`), which the repo itself pins.

## Output per finding

- The workflow and job, with `file:line`
- Evidence: the actionlint line, or the quoted YAML
- Severity: per the report-format skill's rubric, or as step 2 sets it
- Fix: the replacement YAML

## Output file

Load the report-format skill and use its format. Write to the path `kit scratch-dir ci <target-slug>` prints.

Arguments: `$ARGUMENTS`
