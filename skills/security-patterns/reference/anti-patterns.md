# Anti-patterns

Severity rubric: [../SKILL.md](../SKILL.md#severity-rubric).

- User input concatenated or interpolated into a SQL string: `failure`. Use the ORM query API or parameterized queries only, even for admin-only input.
- User-controlled HTML injected into the DOM (React's innerHTML prop, Vue's `v-html`, or an `innerHTML` assignment): `failure`. Render it as plain text, or sanitize it first with a library that parses the full HTML spec, such as DOMPurify. Framework details are in `reference/frontend.md`.
- Session tokens or JWTs in `localStorage`: `failure`. Put session tokens in `HttpOnly; Secure; SameSite=Lax` cookies. A bearer JWT may live in memory (module scope) if the session is short-lived.
- A state-changing endpoint (POST, PUT, PATCH, DELETE) that accepts session-cookie auth without a CSRF token or a `SameSite=Strict` cookie: `failure`. A JSON-only API that rejects `application/x-www-form-urlencoded` is partially mitigated, but not fully protected on older browsers.
- A CSP with `unsafe-inline` or `unsafe-eval`: `warning`. Replace `unsafe-inline` with a nonce or hash-based policy. Replace `unsafe-eval` by refactoring code that builds and runs strings as code at runtime into static functions.
- `Access-Control-Allow-Origin: *` on an endpoint that reads or mutates user data: `warning`. Lock it to the specific list of trusted origins. A wildcard is fine only on a truly public, unauthenticated, read-only API.
- Hardcoded API keys, tokens, or passwords in source, or a committed `.env` file: `failure`. Use a secrets manager or an environment-injection pipeline, add `.env*` to `.gitignore`, and scan with `gitleaks` in CI.
- An auth or session cookie without `HttpOnly` or `Secure`: `warning`. Set both flags, plus `SameSite=Lax` at minimum.
