# Backend (Django)

Settings (`DEBUG`, `SECRET_KEY`, `ALLOWED_HOSTS`, HTTPS headers), template escaping (`|safe`, `mark_safe`), CSRF, and DRF permissions are django-patterns' and drf-patterns' references; review against those.

## Views and middleware

- CSP on Django 6.0+: use the built-in `django.middleware.csp.ContentSecurityPolicyMiddleware` with `SECURE_CSP` / `SECURE_CSP_REPORT_ONLY` over a third-party CSP package. Templates get the nonce as `{{ csp_nonce }}` through the `csp()` context processor.
- DB credentials or API keys in the repo: `failure`. Check `.env` handling, and `env.example` vs `.env`.
- Middleware order: `SecurityMiddleware` first, `SessionMiddleware` before `AuthenticationMiddleware`, and `CsrfViewMiddleware` before views that mutate.

## ORM and query

- `raw()` or `extra()`: parameterize every input.
- Review for N+1 queries and accidental data exposure, such as `.values()` without filtering or an overly broad `select_related()`.
- `.filter(**request.GET)`: never pass arbitrary kwargs to `filter()`.

## Auth and permissions

- Before an edit or delete, check the object-level permission: does the user own this object?
- Password storage: the default hashers are fine. Review any custom implementation.

## File uploads

- Validate extension, MIME type, and content, and serve uploads from a non-executable location.
- Set size limits at both the web server and the Django layer.
