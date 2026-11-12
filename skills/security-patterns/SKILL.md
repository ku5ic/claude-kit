---
name: security-patterns
description: Security checks for XSS, injection, auth, sessions, CSRF, CSP, secrets, and CVEs. Load before writing or reviewing code that handles auth, secrets, or user input, however small.
---

# Security patterns

Apply the references that match the detected stack. The severity rubric below applies to all of them.

## Severity rubric

- `failure`: exploitable, a direct leak of credentials, or missing CSRF on an authenticated mutating endpoint.
- `warning`: mitigated but weak (such as a CSP with `unsafe-inline`), or a pattern that becomes exploitable if misused.
- `info`: a hardening opportunity, not exploitable now.

## Reference files

Loading this skill isn't enough. Before your next edit, read the file of every row that matches it. The rules live in those files, and this one only points at them.

| Before                                                                                        | Read                                                       |
| --------------------------------------------------------------------------------------------- | ---------------------------------------------------------- |
| Writing or reviewing code that handles auth, secrets, or input, or rating a severity          | [reference/anti-patterns.md](reference/anti-patterns.md)   |
| Writing frontend code that renders user input, stores tokens, handles sessions, or sets a CSP | [reference/frontend.md](reference/frontend.md)             |
| Writing Django code that handles auth, queries, permissions, uploads, or middleware           | [reference/backend-django.md](reference/backend-django.md) |
| Writing Express, Fastify, or Nest code that handles auth, headers, or CSRF                    | [reference/backend-node.md](reference/backend-node.md)     |
| Auditing, adding, or updating a dependency                                                    | [reference/dependencies.md](reference/dependencies.md)     |
| Handling a secret, scanning for leaked secrets, or rotating a credential                      | [reference/secrets.md](reference/secrets.md)               |
