---
name: security-patterns
description: Security checks for XSS, injection, auth, sessions, CSRF, CSP, secrets, and CVEs. Use when writing or reviewing code that handles auth, secrets, or user input, or for security questions.
---

# Security patterns

Apply the references that match the detected stack. The severity rubric below applies to all of them.

## Severity rubric

- `failure`: exploitable, a direct leak of credentials, or missing CSRF on an authenticated mutating endpoint.
- `warning`: mitigated but weak (such as a CSP with `unsafe-inline`), or a pattern that becomes exploitable if misused.
- `info`: a hardening opportunity, not exploitable now.

## Reference files

Read every file in this table as soon as this skill loads. A hook blocks every tool call until you have.

| Before                                                                                        | Read                                                       |
| --------------------------------------------------------------------------------------------- | ---------------------------------------------------------- |
| Reviewing code for security flaws, or rating a finding's severity                             | [reference/anti-patterns.md](reference/anti-patterns.md)   |
| Writing frontend code that renders user input, stores tokens, handles sessions, or sets a CSP | [reference/frontend.md](reference/frontend.md)             |
| Writing Django code that handles auth, queries, permissions, uploads, or middleware           | [reference/backend-django.md](reference/backend-django.md) |
| Writing Express, Fastify, or Nest code that handles auth, headers, or CSRF                    | [reference/backend-node.md](reference/backend-node.md)     |
| Auditing, adding, or updating a dependency                                                    | [reference/dependencies.md](reference/dependencies.md)     |
| Handling a secret, scanning for leaked secrets, or rotating a credential                      | [reference/secrets.md](reference/secrets.md)               |
