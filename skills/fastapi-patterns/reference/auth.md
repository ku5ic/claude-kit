# Auth

All security classes import from `fastapi.security`.

- A bearer token flow: `oauth2_scheme = OAuth2PasswordBearer(tokenUrl="token")`, where `tokenUrl` matches the token endpoint's path. The endpoint takes `Annotated[OAuth2PasswordRequestForm, Depends()]` and returns `Token(access_token=..., token_type="bearer")`.
- Protected routes: depend on a `get_current_user(token: Annotated[str, Depends(oauth2_scheme)])` that runs `jwt.decode(token, SECRET_KEY, algorithms=[ALGORITHM])` and loads the user from the `sub` claim.
- A failed login, an `InvalidTokenError`, a missing `sub`, or an unknown user raises 401 with `headers={"WWW-Authenticate": "Bearer"}`.
- Passwords: hash with `pwdlib`'s `PasswordHash.recommended()` (Argon2). Never store plaintext.
- Timing: on an unknown username, still call `verify_password`, against a dummy hash.
- An API key: `APIKeyHeader(name="X-API-Key")` as a dependency that checks the key and raises 403 on a bad one. Read keys from a header, not `APIKeyQuery(name=...)`; `APIKeyCookie(name=...)` reads a cookie.
- HTTP Basic: `HTTPBasic` for internal or development endpoints only, always over HTTPS. Compare username and password with `secrets.compare_digest()` on bytes, and raise 401 with `headers={"WWW-Authenticate": "Basic"}` on a mismatch.
- Optional auth: `OAuth2PasswordBearer(tokenUrl="token", auto_error=False)`. With no credentials, the dependency gets `None` instead of a 401, so type the token `str | None`.
