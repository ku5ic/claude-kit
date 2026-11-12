# Backend (Node APIs: Express, Fastify, Nest)

- Body parsing: set a size limit.
- Validate input at route entry with zod, joi, or a fastify schema, never inside handlers.
- SQL and ORM: parameterized queries only. Flag string concatenation into queries.
- Never leak stack traces to clients in production error responses.
- Rate-limit auth endpoints, password reset, and account enumeration vectors.
- Set security headers with Helmet or an equivalent.
- CORS: an explicit origin list, never `*` with credentials.

## CSRF on JSON-only APIs

A cookie-authenticated JSON API layers these defenses, per the OWASP CSRF cheat sheet:

- On every state-changing request, verify that the `Origin` header matches the expected origin.
- Require a custom header, such as `X-Requested-With`.
- Set `SameSite=Lax` on the auth cookie as defense in depth. It's never the primary control or a complete CSRF defense on its own.

A bearer-token API (the token in `Authorization: Bearer ...`, not a cookie) isn't directly CSRF-vulnerable. Review it for token theft (XSS reading `localStorage`) and replay instead.
