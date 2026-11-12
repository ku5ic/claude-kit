# Settings

## Split settings

- Settings: a package with one file per environment (`base.py`, `dev.py`, `prod.py`, `test.py`), selected at runtime with `DJANGO_SETTINGS_MODULE=myproject.settings.prod`.

## Production

- Production: `DEBUG = False`.
- `ALLOWED_HOSTS`: explicit, such as `os.environ["DJANGO_ALLOWED_HOSTS"].split(",")`. Django raises `SuspiciousOperation` on an unrecognized `Host` header. With `DEBUG = True` and an empty list, it validates against `['.localhost', '127.0.0.1', '[::1]']`.
- `SECRET_KEY`: from the environment (`os.environ["DJANGO_SECRET_KEY"]`), never from code.

## HTTPS

- Serving over HTTPS in production: `SECURE_HSTS_SECONDS = 31536000` (one year), and `True` for `SECURE_SSL_REDIRECT`, `SECURE_HSTS_INCLUDE_SUBDOMAINS`, `SESSION_COOKIE_SECURE`, and `CSRF_COOKIE_SECURE`.
- Behind a proxy: `SECURE_PROXY_SSL_HEADER = ("HTTP_X_FORWARDED_PROTO", "https")`.
- `SECURE_HSTS_PRELOAD = True`: only once the site is permanently HTTPS. It adds the `preload` directive, does nothing unless `SECURE_HSTS_SECONDS` is non-zero, and submits nothing. The directive makes the site eligible for a browser preload list, and a listed site is hard to get off it.

## Environment variables

- Typed values and database URLs from the environment: `django-environ` (0.14.0), as in `env = environ.Env(DEBUG=(bool, False))`, `environ.Env.read_env(".env")`, and `DATABASES = {"default": env.db()}`.
- `python-decouple` or raw `os.environ` work too. The pattern matters more than the library.
