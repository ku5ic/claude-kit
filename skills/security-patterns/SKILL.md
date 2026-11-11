---
name: security-patterns
description: Security checks for XSS, injection, auth, sessions, CSRF, CSP, secrets, and CVEs. Use when writing or reviewing code that handles auth, secrets, or user input, or for security questions.
---

# Security patterns

Apply the references that match the detected stack. Severity rubric below applies cross-cuttingly.

## Severity rubric

- `failure`: exploitable, or direct leak of credentials, or missing CSRF on authenticated mutating endpoint.
- `warning`: mitigated but weak (e.g. CSP present but with `unsafe-inline`), or pattern that will become exploitable if misused.
- `info`: hardening opportunity, not currently exploitable.

## Reference files

| File                                                       | Covers                                                                                                      |
| ---------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| [reference/anti-patterns.md](reference/anti-patterns.md)   | Cross-cutting severity-labeled anti-patterns: SQLi, XSS injection, CSRF, secrets                            |
| [reference/frontend.md](reference/frontend.md)             | XSS, Next.js client/server boundary, auth/sessions, JWT vs sessions, strict CSP (nonce/strict-dynamic/hash) |
| [reference/backend-django.md](reference/backend-django.md) | ORM safety, views/middleware, auth, templates, file uploads                                                 |
| [reference/backend-node.md](reference/backend-node.md)     | Express/Fastify/Nest hardening, CSRF on JSON-only APIs                                                      |
| [reference/dependencies.md](reference/dependencies.md)     | Lockfile audit vs OSV-Scanner, SLSA build levels                                                            |
| [reference/secrets.md](reference/secrets.md)               | Detection (gitleaks, trufflehog, push protection), rotation cadence                                         |
