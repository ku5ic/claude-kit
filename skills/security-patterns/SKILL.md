---
name: security-patterns
description: Security checklist covering XSS, injection, authentication, authorization, sessions, CSRF, CSP, secrets, dependency CVEs, input validation, and severity calls. Use whenever the project includes auth code, session handling, environment variable reads, user input handling, route handlers, server actions, middleware, or external API calls, OR the user asks about security, hardening, vulnerabilities, auth, authentication, authorization, sessions, cookies, XSS, CSRF, SQL injection, secrets, environment variables, CSP, headers, or reviews changes that touch user input, auth, or external data, even if "security" is not mentioned by name.
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

## References

- OWASP Top 10: https://owasp.org/www-project-top-ten/
- OWASP Cheat Sheet Series: https://cheatsheetseries.owasp.org/
- OWASP ASVS: https://owasp.org/www-project-application-security-verification-standard/
- MDN Web Security: https://developer.mozilla.org/en-US/docs/Web/Security
- MDN CSP: https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP
- WHATWG Fetch (CORS): https://fetch.spec.whatwg.org/
