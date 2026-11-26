---
name: audit-security
description: Security review of a tree: the whole repo, a directory, or a file
argument-hint: "<file, directory, area name, or a link to an external tracker/doc>"
allowed-tools:
  - Bash(git log *)
---

# /audit-security

Security review of a tree: the whole repo, a directory, or a file. A diff's security review is the built-in `/security-review`.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                                  | Read                                                     |
| --------------------------------------- | -------------------------------------------------------- |
| Running the secret scans, steps 3 and 4 | [reference/scanners.md](reference/scanners.md)           |
| Checking authorization, step 7          | [reference/authorization.md](reference/authorization.md) |

## Run

With no target in the arguments, ask for one first, since the auditor can't ask. Resolve a linked ticket or doc as `skills/investigate` step 0 describes, then work this file's procedure on the resolved arguments per `skills/kit-rules/reference/subagents.md` section 4, and write the report yourself. Then run the report-format skill's Runtime checks yourself, and reply per `skills/kit-rules/reference/output.md` section 3.

## Procedure

1. Stack is in the `<repo-context>` block from the `SubagentStart` hook.
2. Load the security-patterns skill. Read its `anti-patterns.md` and `secrets.md`, and every other reference its table names for the detected stack.
3. Secrets in the tree: `gitleaks dir --no-banner --redact <target>`.
4. Secrets in history, for a whole-repo target: `trufflehog git file://. --no-verification --no-update`. Why the flag: [reference/scanners.md](reference/scanners.md).
5. Read the code that takes outside input: routes, handlers, views, serializers, form and upload handling, and anything that builds SQL, shell, HTML, or a URL from it. In a directory, find them with `rg` for the framework's route and handler markers before reading.
6. Check each one against the loaded references: injection, XSS, CSRF, session and token handling, CSP and security headers.
7. Authorization: for each route or handler, name the check that limits who can call it and whose data it returns, per [reference/authorization.md](reference/authorization.md). A mutating or per-user endpoint with no such check is a finding.
8. Dependency CVEs are `/deps`' job. Name it as the next step instead of scanning.

## Not a finding

- A credential-shaped string the code never uses as one, such as a documented placeholder or a test fixture's dummy key. Name it as `info` so the user can confirm, never as a leak.
- Interpolation a framework escapes by default, such as React text or a Django template variable, unless the code turns escaping off (`dangerouslySetInnerHTML`, `|safe`, `mark_safe`).
- A dependency CVE, which `/deps` covers.

## Output per finding

- What it is, with `file:line`
- Evidence: the tool output line, or the quoted code and the input that reaches it
- Severity: the security-patterns rubric
- Fix: the concrete first step

A gitleaks or trufflehog hit is reported redacted, never with the secret.

## Output file

Load the report-format skill and use its format. Write to the path `kit scratch-dir security <target-slug>` prints.

Arguments: `$ARGUMENTS`
