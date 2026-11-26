---
name: audit-debt
description: Technical debt inside a target's modules, ranked by severity, with a remediation path
argument-hint: "<file, directory, area name, or a link to an external tracker/doc>"
allowed-tools:
  - Bash(git log *)
---

# /audit-debt

Surface technical debt inside the target's modules, with severity and remediation path. Coupling, layering, and cycles between modules are `/audit-arch`.

## Run

With no target in the arguments, ask for one first, since the auditor can't ask. Resolve a linked ticket or doc as `skills/investigate` step 0 describes, then work this file's procedure on the resolved arguments per `skills/kit-rules/reference/subagents.md` section 4, and write the report yourself. Then run the report-format skill's Runtime checks yourself, and reply per `skills/kit-rules/reference/output.md` section 3.

## Procedure

1. Stack is in the `<repo-context>` block from the `SubagentStart` hook.
2. Load the patterns skill for the detected stack (react-patterns, django-patterns, etc.) for the anti-pattern reference.
3. Read the target.
   1. If $ARGUMENTS is a directory, run `tokei --files --sort code <path>` to get accurate line counts per language and per file.
   2. Read the 5 files with the highest code-line counts (excluding tests, generated files, and lockfiles).
   3. Read any `index.ts`, `main.py`, `urls.py`, `routes.ts` equivalents.
4. Run `git log --oneline -20` on the target to see recent churn. High-churn files are candidates for higher-priority debt.
5. Collect deliberate shortcuts: `rg -n 'shortcut:' <target>`. Each is a finding in its own right. Its stated ceiling is the problem and its upgrade path is the remediation. A shortcut in a high-churn file, or one whose ceiling the code already exceeds, ranks higher.
6. Evaluate across these categories. Skip categories with no findings. Do not pad.

### Categories

- **Type safety**: escape hatches, runtime assumptions not in types, unsafe casts, `any` in public surface
- **State management**: local state that should be lifted, global state that should be local, derived state stored instead of computed
- **Side effects**: unguarded async, missing cleanup, implicit ordering, hidden I/O
- **Scalability**: works now, breaks at scale (data size, user count, team size)
- **Testing debt**: core paths with no tests, brittle tests, untestable designs
- **Security debt**: authn/authz gaps, input validation, exposed secrets, dependency CVEs (if easy to check)
- **Observability debt**: silently swallowed errors, no logging on critical paths, no telemetry where it matters
- **Dead code and duplication**: unused exports, and copies of one thing along a stable axis (copies with divergent lifecycles are correct, per `skills/kit-rules/reference/change.md` section 8). Each finding names the pattern `skills/engineering-fundamentals/reference/design-patterns.md` gives for its signal.
- **Build and dependency health**: deprecated APIs, ejected config, outdated patterns, unmaintained deps
- **Metric thresholds breached**: the numbers in `skills/engineering-fundamentals/reference/metric-thresholds.md`; use `tokei` for size and a quick scan for the rest. Each is a smell, not a failure; the audit's value is correlating these with high churn from `git log`.

## Not a finding

- A `shortcut:` comment whose ceiling the code hasn't reached and whose file barely changes. It stays `info` at most.
- Two copies with different lifecycles, per `skills/kit-rules/reference/change.md` section 8.
- A metric threshold breached in a file nobody changes, with no bug history in `git log`.

## Severity anchors

- `failure`: the debt is causing harm now, such as a swallowed error on a payment path, or a core path with no test that broke in the last 20 commits.
- `warning`: it will cost the next change here, such as a 900-line file in the top churn, or duplicated logic already edited in one copy only.
- `info`: worth knowing, with no cost yet.

## Output per finding

- What it is, with `file:line`
- Evidence: what was checked and what it showed (a `tokei` size, a churn count from `git log`, the quoted line)
- Why it matters: scale, maintainability, correctness, security, performance, or operability
- Severity: per the report-format skill's rubric; `failure` here means an active liability, not a future risk
- Fix: the concrete first step, and its effort: small (hours), medium (days), or large (weeks)

## Output file

Load the report-format skill and use its format. Write to the path `kit scratch-dir debt <target-slug>` prints.

Sort findings by severity, then by effort (smallest first within each severity) so the quick wins are visible at the top.
