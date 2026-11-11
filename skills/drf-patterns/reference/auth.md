# Authentication

## Built-in schemes

- Enable a scheme in `REST_FRAMEWORK["DEFAULT_AUTHENTICATION_CLASSES"]`, as in `"rest_framework.authentication.TokenAuthentication"`.
- `TokenAuthentication` sends a token in the `Authorization: Token <key>` header, stored in the database, one per user. Use it for mobile and desktop clients, over HTTPS in production.
- Generate tokens with `manage.py drf_create_token <username>`, the built-in `obtain_auth_token` view, or a signal on user creation.
- `SessionAuthentication` uses Django's session backend. Use it for same-origin AJAX clients sharing the browser session; unsafe methods (POST, PUT, PATCH, DELETE) need a CSRF token.
- `BasicAuthentication`: only for development or internal tools, never over plain HTTP.

## JWT via djangorestframework-simplejwt

- JWT isn't built into DRF; the DRF docs name `djangorestframework-simplejwt`, whose class is `rest_framework_simplejwt.authentication.JWTAuthentication`.
- simplejwt issues a short-lived access token (typically 5 minutes) and a longer-lived refresh token. The access token is stateless; simplejwt's blacklist app can blacklist the refresh token.
- A DRF token is one long-lived token that needs a database lookup per request and is revoked by deleting its row.
- A JWT access token needs no database lookup per request but can't be revoked before expiry without a blacklist. The refresh token needs a database lookup.
- Pick DRF tokens for simplicity and easy revocation, JWT for stateless horizontal scaling or cross-service authentication.

## Throttle auth endpoints

- Any endpoint that issues or refreshes tokens gets a tight rate limit. See `throttling-and-pagination.md`.
