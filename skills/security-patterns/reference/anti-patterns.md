# Anti-patterns

Severity rubric:

- `failure`: exploitable, or direct leak of credentials, or missing CSRF on authenticated mutating endpoint.
- `warning`: mitigated but weak, or pattern that will become exploitable if misused.
- `info`: hardening opportunity, not currently exploitable.

## String-interpolated SQL

`failure`. Concatenating user input into a SQL string bypasses parameterization and enables injection. Use the ORM query API or parameterized queries exclusively. Even "safe" admin-only inputs should be parameterized; privilege does not remove the class.

## Raw HTML injection with unsanitized input (XSS)

`failure`. Injecting user-controlled HTML directly into the DOM (React's innerHTML prop, Vue's `v-html`, or direct `innerHTML` assignment) executes scripts in the same origin. Either render content as plain text, or sanitize with a library that parses the full HTML spec (DOMPurify) before injecting. See `reference/frontend.md` for framework-specific details.

## Storing session tokens or JWTs in `localStorage`

`failure`. `localStorage` is readable by any JavaScript on the page. One XSS vulnerability exposes every stored token. Use `HttpOnly; Secure; SameSite=Lax` cookies for session tokens. JWTs intended as bearer tokens may live in memory (module scope) if the session is short-lived.

## Authenticated mutating endpoint without CSRF protection

`failure`. A state-changing endpoint (POST, PUT, PATCH, DELETE) that accepts session-cookie auth without a CSRF token or `SameSite=Strict` cookie can be triggered cross-origin. JSON-only APIs that reject `application/x-www-form-urlencoded` are partially mitigated but not fully protected on older browsers.

## CSP with `unsafe-inline` or `unsafe-eval`

`warning`. Both directives defeat the main XSS mitigation that CSP provides. Replace `unsafe-inline` with a nonce or hash-based policy. Replace `unsafe-eval` by refactoring any code that constructs and executes strings as code at runtime into static functions.

## Overly permissive CORS (`Access-Control-Allow-Origin: *`)

`warning`. A wildcard origin is acceptable for truly public, unauthenticated read-only APIs, but is incorrect for any endpoint that reads or mutates user data. Lock `Access-Control-Allow-Origin` to the specific trusted origin list.

## Secrets in source code or committed `.env` files

`failure`. Hardcoded API keys, tokens, and passwords are captured in git history and visible to everyone with repo access. Use a secrets manager or environment-injection pipeline. Add `.env*` to `.gitignore` and scan with `gitleaks` in CI.

## Missing `HttpOnly` and `Secure` flags on auth cookies

`warning`. Without `HttpOnly`, JavaScript can read the cookie; without `Secure`, the cookie is sent over plain HTTP. Auth and session cookies must set both flags plus `SameSite=Lax` at minimum.

## References

- OWASP SQL Injection: https://owasp.org/www-community/attacks/SQL_Injection
- OWASP XSS Prevention: https://cheatsheetseries.owasp.org/cheatsheets/Cross_Site_Scripting_Prevention_Cheat_Sheet.html
- OWASP CSRF Prevention: https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html
- OWASP Session Management: https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html
- CSP nonce/hash guidance: https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP
- DOMPurify: https://github.com/cure53/DOMPurify
