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

| File                                                       | Covers                                                                                                      |
| ---------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| [reference/anti-patterns.md](reference/anti-patterns.md)   | Eight anti-patterns with severity calls: SQLi, XSS, token storage, CSRF, CSP, CORS, secrets, cookie flags   |
| [reference/frontend.md](reference/frontend.md)             | XSS, Next.js client/server boundary, auth/sessions, JWT vs sessions, strict CSP (nonce/strict-dynamic/hash) |
| [reference/backend-django.md](reference/backend-django.md) | Django 6.0+ CSP, middleware order, ORM safety, object-level permissions, file uploads                       |
| [reference/backend-node.md](reference/backend-node.md)     | Express/Fastify/Nest hardening, CSRF on JSON-only APIs                                                      |
| [reference/dependencies.md](reference/dependencies.md)     | Lockfile audit vs OSV-Scanner, SLSA build levels                                                            |
| [reference/secrets.md](reference/secrets.md)               | Detection (gitleaks, trufflehog, push protection), rotation, rotation cadence                               |
