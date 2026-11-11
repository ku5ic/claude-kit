# Frontend (React, Next.js)

## XSS and injection

- `dangerouslySetInnerHTML`: check every occurrence. Sanitize with DOMPurify or an equivalent, or remove it.
- URLs in `href`, `src`, `action`, `formAction`: validate the scheme. Never allow `javascript:` URLs from user input or unchecked `data:` URIs.
- Check `router.push(userValue)`, `window.location = userValue`, and server action redirects with user input for open redirects, and validate the target against an allowlist.
- Check for template injection: variables interpolated into raw HTML strings, and Markdown renderers without sanitization.

## Client / server boundary (Next.js)

next-app-router-patterns owns these: `NEXT_PUBLIC_*` secrets (`reference/runtime.md`), Server Function auth and input validation (`reference/server-actions.md`), and server-only APIs (`reference/server-and-client.md`). Route handlers too: validate body and query with a schema (Zod, Valibot), and don't trust the `Content-Type` header.

## Auth and sessions

- Session cookies: `httpOnly`, `secure`, `sameSite`. Audit Set-Cookie headers or middleware.
- Cookie-based auth: flag a mutation that has no CSRF defense (double-submit cookie, origin check, or `sameSite` strict).
- JWT: verify the signature, and check `exp`, `iss`, `aud`. Never trust decoded claims as auth.
- A state-changing GET request is a defect. RFC 9110 makes GET, HEAD, and OPTIONS safe methods.
- Session fixation: always regenerate the session ID at the auth boundary (login, privilege change).
- Single-tenant first-party login flows: use a plain server-managed session, not JWT. Keep JWT for genuinely stateless contexts, such as token federation across services.
- When JWT is the right call: rotate the refresh token on every use, revoke any reused refresh token as theft, and persist per-token state for revocation.
- JWT vulnerabilities to flag:
  - `alg: none` accepted: `failure`.
  - A weak HMAC secret that allows offline cracking: `failure`.
  - Missing `exp`: `failure`.
  - No revocation mechanism on a high-stakes session: at least `warning`.

## Content Security Policy

- Flag `unsafe-inline` in `script-src`, `unsafe-eval`, and a missing CSP header.
- Use a strict CSP built on nonces or hashes. For a Next.js App Router app, follow the pattern in the Next.js docs.
- Server-rendered apps: nonce-based. Generate a fresh, unpredictable random value per response, and put it in `script-src 'nonce-XXXX'` and on every `<script nonce="XXXX">` tag.
- Bundler-emitted dynamic loads: pair the nonce or hash with `strict-dynamic`, which lets a trusted script load further scripts that carry no nonce of their own.
- Static pages: hash-based (`sha256-...`, `sha384-...`, `sha512-...`). External scripts must carry the matching `integrity` attribute.
- Migrating off `unsafe-inline`: add the nonce or hash to the same directive, and keep `unsafe-inline` only as a fallback for older browsers. Browsers ignore `unsafe-inline` in a directive that has a nonce or hash.
- A minimal strict CSP: `script-src 'nonce-{RANDOM}' 'strict-dynamic'; object-src 'none'; base-uri 'none';`.
