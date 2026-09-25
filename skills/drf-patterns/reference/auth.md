# Authentication

## Built-in schemes

**TokenAuthentication**: simple token in `Authorization: Token <key>` header. Tokens are stored in the database (one per user). Suitable for mobile and desktop clients. Requires HTTPS in production (token travels in every request header).

```python
REST_FRAMEWORK = {
    "DEFAULT_AUTHENTICATION_CLASSES": [
        "rest_framework.authentication.TokenAuthentication",
    ]
}
```

Generate tokens: `manage.py drf_create_token <username>`, or via the built-in `obtain_auth_token` view, or automatically via signals on user creation.

**SessionAuthentication**: uses Django's session backend. Appropriate for same-origin AJAX clients sharing the browser session. Requires CSRF tokens on unsafe methods (POST, PUT, PATCH, DELETE).

**BasicAuthentication**: HTTP Basic Auth. Only for development or internal tools. Never use over plain HTTP.

## JWT via djangorestframework-simplejwt

DRF does not include JWT out of the box. `djangorestframework-simplejwt` is the recommended third-party library (cited in DRF docs).

```python
REST_FRAMEWORK = {
    "DEFAULT_AUTHENTICATION_CLASSES": [
        "rest_framework_simplejwt.authentication.JWTAuthentication",
    ]
}
```

Key properties: access token (short-lived, typically 5 minutes) + refresh token (longer-lived). The access token is stateless; the refresh token can be blacklisted via simplejwt's blacklist app.

JWT vs DRF token:

- DRF token: single long-lived token, requires database lookup per request, easy to revoke by deleting the token row.
- JWT: stateless access token (no DB lookup per request), but cannot be revoked until expiry without a blacklist. The refresh token requires a DB lookup.

Choose DRF tokens for simplicity and easy revocation. Choose JWT when you need stateless horizontal scaling or cross-service authentication.

## Throttle auth endpoints

Apply tight rate limits to any endpoint that issues or refreshes tokens. See `throttling-and-pagination.md`.

## References

- https://www.django-rest-framework.org/api-guide/authentication/
- https://django-rest-framework-simplejwt.readthedocs.io/en/latest/
