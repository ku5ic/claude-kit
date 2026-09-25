# Auth

## Contents

- [OAuth2 with JWT](#oauth2-with-jwt)
- [API key (header)](#api-key-header)
- [HTTP Basic](#http-basic)
- [Optional authentication](#optional-authentication)
- [References](#references)

All security classes import from `fastapi.security`.

## OAuth2 with JWT

The `OAuth2PasswordBearer` scheme declares a Bearer token flow. The `tokenUrl` must match the path of your token endpoint.

```python
from datetime import datetime, timedelta, timezone
from typing import Annotated
from fastapi import Depends, FastAPI, HTTPException, status
from fastapi.security import OAuth2PasswordBearer, OAuth2PasswordRequestForm

oauth2_scheme = OAuth2PasswordBearer(tokenUrl="token")

@app.post("/token")
def login(form_data: Annotated[OAuth2PasswordRequestForm, Depends()]) -> Token:
    user = authenticate_user(form_data.username, form_data.password)
    if not user:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Incorrect username or password",
            headers={"WWW-Authenticate": "Bearer"},
        )
    access_token = create_access_token(
        data={"sub": user.username},
        expires_delta=timedelta(minutes=ACCESS_TOKEN_EXPIRE_MINUTES),
    )
    return Token(access_token=access_token, token_type="bearer")
```

Verify the token in a dependency used by protected routes:

```python
def get_current_user(token: Annotated[str, Depends(oauth2_scheme)]) -> User:
    credentials_exception = HTTPException(
        status_code=status.HTTP_401_UNAUTHORIZED,
        detail="Could not validate credentials",
        headers={"WWW-Authenticate": "Bearer"},
    )
    try:
        payload = jwt.decode(token, SECRET_KEY, algorithms=[ALGORITHM])
        username: str | None = payload.get("sub")
        if username is None:
            raise credentials_exception
    except InvalidTokenError:
        raise credentials_exception
    user = get_user(username=username)
    if user is None:
        raise credentials_exception
    return user
```

Password hashing: use `pwdlib` with `PasswordHash.recommended()` (Argon2). Never store plaintext passwords.

Timing attack prevention: always call `verify_password` even for non-existent users (verify against a dummy hash) so response time does not reveal whether the username exists.

## API key (header)

`APIKeyHeader` extracts a key from a named request header. Validate the key in the dependency:

```python
from fastapi.security import APIKeyHeader

api_key_header = APIKeyHeader(name="X-API-Key")

def verify_api_key(api_key: Annotated[str, Depends(api_key_header)]) -> str:
    if api_key not in valid_api_keys:
        raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail="Invalid API key")
    return api_key

@app.get("/data/")
def get_data(key: Annotated[str, Depends(verify_api_key)]):
    return {"data": "..."}
```

`APIKeyQuery(name="api_key")` extracts from a query parameter. `APIKeyCookie(name="session")` extracts from a cookie. Prefer header over query: query parameters appear in server logs and browser history.

## HTTP Basic

Use `HTTPBasic` only for internal or development endpoints. Always require HTTPS. Use `secrets.compare_digest()` to prevent timing attacks:

```python
import secrets
from fastapi.security import HTTPBasic, HTTPBasicCredentials

security = HTTPBasic()

def verify_basic(credentials: Annotated[HTTPBasicCredentials, Depends(security)]) -> str:
    ok_user = secrets.compare_digest(
        credentials.username.encode(), b"admin"
    )
    ok_pass = secrets.compare_digest(
        credentials.password.encode(), b"secret"
    )
    if not (ok_user and ok_pass):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            headers={"WWW-Authenticate": "Basic"},
        )
    return credentials.username
```

## Optional authentication

Set `auto_error=False` to make auth optional. The dependency returns `None` instead of raising 401 when no credentials are provided:

```python
optional_oauth2 = OAuth2PasswordBearer(tokenUrl="token", auto_error=False)

def get_optional_user(
    token: Annotated[str | None, Depends(optional_oauth2)],
) -> User | None:
    if token is None:
        return None
    return get_current_user(token)
```

## References

- https://fastapi.tiangolo.com/tutorial/security/oauth2-jwt/
- https://fastapi.tiangolo.com/advanced/security/http-basic-auth/
- https://fastapi.tiangolo.com/reference/security/
