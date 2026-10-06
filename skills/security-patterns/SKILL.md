---
name: security-patterns
description: Security checklist covering XSS, injection, authentication, authorization, sessions, CSRF, CSP, secrets, dependency CVEs, input validation, and severity calls. Use when writing or reviewing code that handles auth, sessions, secrets, user input, or external data, or when the user asks about security, hardening, vulnerabilities, authentication, authorization, cookies, XSS, CSRF, SQL injection, CSP, or headers, even if "security" is not mentioned by name. Not loaded just because a project has such code.
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

## Version notes

Checked: 2026-09-26 against https://slsa.dev/spec/v1.2/build-track-basics, https://docs.djangoproject.com/en/stable/ref/csp/, https://docs.npmjs.com/cli/v12/commands/npm-audit

- SLSA 1.2: current spec; 1.0 retired. Build track is L0 (no requirements) through L3.
- Django 6.0: built-in CSP middleware (`SECURE_CSP`, `csp_nonce`); 6.1 adds the `{% csp_nonce_attr %}` tag.
- npm 12 (12.1.0): current CLI docs; v10 and v11 docs marked Legacy.
