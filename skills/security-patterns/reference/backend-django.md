# Backend (Django)

## ORM and query

- Raw SQL via `raw()` or `extra()`: every use must parameterize inputs.
- N+1 and accidental data exposure: `.values()` without filtering, overly broad `select_related()`.
- `.filter(**request.GET)`: never. Arbitrary kwargs to filter is dangerous.

## Views and middleware

- CSRF: `@csrf_exempt` on mutating views is a failure unless the view is read-only or explicitly API token authenticated.
- `DEBUG = True` in committed settings: `failure`.
- `SECRET_KEY`, DB credentials, API keys in repo: `failure`. Check `.env` handling, `env.example` vs `.env`.
- `ALLOWED_HOSTS` wildcard in production settings: `failure`.
- Middleware order: `SecurityMiddleware` first, `SessionMiddleware` before `AuthenticationMiddleware`, `CsrfViewMiddleware` before views that mutate.

## Auth and permissions

- DRF viewsets: `permission_classes` set explicitly. Default `AllowAny` is a failure for write endpoints.
- Object-level permissions: does user own this object before edit or delete?
- Password storage: default hashers OK. Custom implementations need review.

## Templates

- `{% autoescape off %}` or `|safe`: every use. Must be on trusted content only.
- `mark_safe()` on user-derived data: `failure`.

## File uploads

- Validate extension, MIME type, and content. Serve from non-executable location.
- Size limits at web server and Django layer.

## References

- Django security topics: https://docs.djangoproject.com/en/stable/topics/security/
- Django deployment checklist: https://docs.djangoproject.com/en/stable/howto/deployment/checklist/
- DRF permissions: https://www.django-rest-framework.org/api-guide/permissions/
