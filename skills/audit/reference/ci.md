# /audit ci

Hardening review of CI workflows. Arguments: `<workflow file, or none for every file in .github/workflows>`.

## Prerequisites

- No `.github/workflows` directory: stop and say so.

## Procedure

1. Run `actionlint` on the target workflows. Each error is a finding.
2. Read each workflow and check:
   - **Pinning**: every `uses:` of an action outside the repo is pinned to a full commit SHA, with the tag in a comment. A tag or branch is a finding; a first-party `actions/*` tag is a `warning`, a third-party one a `failure`.
   - **Permissions**: a top-level or per-job `permissions:` block granting only what the jobs use. None at all means the token's default scope, a `warning`.
   - **Untrusted triggers**: `pull_request_target` or `workflow_run` that checks out or runs the PR's code is a `failure`.
   - **Injection**: `${{ github.event.* }}`, `${{ github.head_ref }}`, or another attacker-set value interpolated into `run:` is a `failure`. The fix passes it through `env:`.
   - **Secrets**: a secret echoed, written to a file or artifact, or passed to a step that runs PR code.
   - **Self-hosted runners** on a workflow that public PRs can trigger.
3. Infra files the jobs deploy are `audit infra`'s job.

## Output per finding

- The workflow and job, with `file:line`
- Evidence: the actionlint line, or the quoted YAML
- Severity: per the report-format skill's rubric, or as step 2 sets it
- Fix: the replacement YAML

## Output file

Load the report-format skill and use its format. Write to the path `kit scratch-dir ci <target-slug>` prints.
