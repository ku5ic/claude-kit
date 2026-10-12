# Backend (Django)

## ORM and query

- Raw SQL via `raw()` or `extra()`: every use must parameterize inputs.
- N+1 and accidental data exposure: `.values()` without filtering, overly broad `select_related()`.
- `.filter(**request.GET)`: never. Arbitrary kwargs to filter is dangerous.

Settings (`DEBUG`, `SECRET_KEY`, `ALLOWED_HOSTS`, HTTPS headers), template escaping (`|safe`, `mark_safe`), CSRF, and DRF permissions are django-patterns' and drf-patterns' references; review against those.

## Views and middleware

- DB credentials and API keys in the repo: `failure`. Check `.env` handling, `env.example` vs `.env`.
- Middleware order: `SecurityMiddleware` first, `SessionMiddleware` before `AuthenticationMiddleware`, `CsrfViewMiddleware` before views that mutate.
- CSP on Django 6.0+: built in via `django.middleware.csp.ContentSecurityPolicyMiddleware` with `SECURE_CSP` / `SECURE_CSP_REPORT_ONLY`; nonces reach templates through the `csp()` context processor as `{{ csp_nonce }}`. Prefer it over a third-party CSP package on 6.0+.

## Auth and permissions

- Object-level permissions: does user own this object before edit or delete?
- Password storage: default hashers OK. Custom implementations need review.

## File uploads

- Validate extension, MIME type, and content. Serve from non-executable location.
- Size limits at web server and Django layer.

## References

- Django security topics: https://docs.djangoproject.com/en/stable/topics/security/
- Django deployment checklist: https://docs.djangoproject.com/en/stable/howto/deployment/checklist/
- DRF permissions: https://www.django-rest-framework.org/api-guide/permissions/
