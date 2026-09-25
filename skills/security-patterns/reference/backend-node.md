# Backend (Node APIs: Express, Fastify, Nest)

- Body parsing: size limits. Default is often no limit.
- Input validation at route entry: zod, joi, or fastify schema. Not inside handlers.
- SQL and ORM: parameterized queries only. Flag string concatenation into queries.
- Error responses: do not leak stack traces to clients in production.
- Rate limiting on auth endpoints, password reset, account enumeration vectors.
- Helmet or equivalent security headers.
- CORS: explicit origin list, not `*` with credentials.

## CSRF on JSON-only APIs

For cookie-authenticated JSON APIs, the OWASP Cheat Sheet recommends defense in layers:

- Verify the `Origin` header matches the expected origin on every state-changing request. The browser sets `Origin` automatically and a script in another origin cannot forge it.
- Require a custom header (e.g. `X-Requested-With`). Browsers do not auto-send custom headers cross-origin without a CORS preflight, which the server can refuse.
- `SameSite=Lax` on the auth cookie as defense in depth, not as the primary control. Lax-mode cookies are still sent with top-level navigations of unsafe methods, so it is not a complete CSRF defense on its own.

A bearer-token API (where the token is in `Authorization: Bearer ...`, not a cookie) is not directly CSRF-vulnerable because the browser does not auto-send `Authorization` headers. The vulnerabilities shift to token theft (XSS extracting localStorage) and replay; neither is a CSRF case.

## References

- OWASP CSRF Cheat Sheet: https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html
- Express security best practices: https://expressjs.com/en/advanced/best-practice-security.html
- Helmet: https://helmetjs.github.io/
- WHATWG Fetch (CORS): https://fetch.spec.whatwg.org/
