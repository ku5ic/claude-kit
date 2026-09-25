# Frontend (React, Next.js)

## Contents

- [XSS and injection](#xss-and-injection)
- [Client / server boundary (Next.js)](#client--server-boundary-nextjs)
- [Auth and sessions](#auth-and-sessions)
- [Content Security Policy](#content-security-policy)

## XSS and injection

- `dangerouslySetInnerHTML`: every occurrence. Must sanitize with DOMPurify or equivalent, or remove.
- URLs in `href`, `src`, `action`, `formAction`: validate scheme. No `javascript:` URLs from user input. No unchecked `data:` URIs.
- Open redirects: any `router.push(userValue)`, `window.location = userValue`, server action redirect with user input. Must validate against an allowlist.
- Template injection: variables interpolated into raw HTML strings, Markdown renderers without sanitization.

## Client / server boundary (Next.js)

- Secrets: `NEXT_PUBLIC_*` is sent to the browser. Anything sensitive must not have this prefix. Audit all `process.env.*` reads in client components.
- Server actions: check `use server` files. Validate input at the action boundary. Assume every arg is user controlled.
- `headers()`, `cookies()`: used only in server components, route handlers, or server actions. Never leaked to client.
- Route handlers: validate body and query with a schema (zod, valibot). Do not trust `Content-Type` header.

## Auth and sessions

- Session cookies: `httpOnly`, `secure`, `sameSite`. Audit Set-Cookie headers or middleware.
- CSRF: double-submit cookie, origin check, or sameSite strict for cookie-based auth. Missing on mutations.
- JWT: verify signature, check `exp`, `iss`, `aud`. Do not trust decoded claims as auth.
- **State-changing GET requests are a defect** (RFC 9110: GET, HEAD, OPTIONS are safe methods; CSRF defenses generally exempt them, so a GET that mutates state has neither protection nor convention on its side).
- **Session fixation**: regenerate the session ID at the auth boundary (login, privilege change). OWASP Session Management cheat sheet calls this out as mandatory.
- **JWT versus server-managed sessions**: OWASP explicitly recommends server-managed sessions for first-party applications that do not need to be fully stateless. JWT belongs in genuinely stateless contexts (token federation across services); for single-tenant first-party login flows, sessions are simpler and safer.
- **Refresh-token rotation**: when JWT is the right call, rotate the refresh token on every use and revoke any reused refresh token (treat reuse as theft). Persist a per-token state for revocation; pure stateless JWT cannot revoke until expiry.
- **JWT vulnerabilities to flag**:
  - `alg: none` attack: `failure`.
  - Weak HMAC secrets allowing offline cracking: `failure`.
  - Missing `exp`: `failure`.
  - Missing revocation mechanism: at least `warning` for any high-stakes session.

## Content Security Policy

- `unsafe-inline` in `script-src`: flag. Indicates a gap.
- `unsafe-eval`: flag.
- Missing CSP header entirely: flag.

### Strict CSP patterns

The current best-practice CSP is "strict CSP" using nonces or hashes per MDN. The Next.js docs document the corresponding pattern for App Router applications.

- **Nonce-based** (recommended for server-rendered apps): the server generates a fresh random value per response and includes it in `script-src 'nonce-XXXX'` and on every `<script nonce="XXXX">` tag. Browser only loads scripts whose nonce matches. The nonce MUST be unique per response and unpredictable.
- **`strict-dynamic`**: paired with a nonce or hash, allows scripts loaded by a trusted script to load further scripts without each carrying its own nonce. Practical for apps with bundler-emitted dynamic loads.
- **Hash-based** (`sha256-...`, `sha384-...`, `sha512-...`): suits static pages because both CSP and content can stay constant. External scripts must include the matching `integrity` attribute.

When a directive contains nonce or hash expressions, browsers IGNORE `unsafe-inline`. That is the migration path: keep `unsafe-inline` only as a fallback for older browsers, and add the nonce or hash to the same directive so modern browsers enforce strict policy.

Example minimal strict CSP:

```
Content-Security-Policy:
  script-src 'nonce-{RANDOM}' 'strict-dynamic';
  object-src 'none';
  base-uri 'none';
```

## References

- OWASP Top 10: https://owasp.org/www-project-top-ten/
- OWASP HTML5 Security: https://cheatsheetseries.owasp.org/cheatsheets/HTML5_Security_Cheat_Sheet.html
- OWASP CSRF Prevention: https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html
- OWASP Session Management: https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html
- OWASP JWT Cheat Sheet: https://cheatsheetseries.owasp.org/cheatsheets/JSON_Web_Token_for_Java_Cheat_Sheet.html
- MDN CSP: https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP
