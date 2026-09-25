# Settings

## Split settings

Separate settings into a package with one file per environment:

```
myproject/settings/
    __init__.py
    base.py      # shared across all environments
    dev.py       # development overrides
    prod.py      # production values
    test.py      # test runner overrides
```

Select at runtime via `DJANGO_SETTINGS_MODULE` env var:

```bash
DJANGO_SETTINGS_MODULE=myproject.settings.prod python manage.py runserver
```

This pattern is from "Two Scoops of Django" (Greenfeld/Greenfeld) and is independently recommended in the django-environ docs and Real Python Django tutorials.

## Production checklist

`DEBUG = False` in production. `DEBUG = True` exposes full tracebacks, SQL query lists, and settings values to any exception view.

`ALLOWED_HOSTS` must be explicit. Django raises `SuspiciousOperation` on requests with unrecognized `Host` headers, but only when `DEBUG = False`.

`SECRET_KEY` comes from env, not from code. A leaked `SECRET_KEY` breaks session security and CSRF protection.

```python
# prod.py
import os
SECRET_KEY = os.environ["DJANGO_SECRET_KEY"]
DEBUG = False
ALLOWED_HOSTS = os.environ["DJANGO_ALLOWED_HOSTS"].split(",")
```

## HTTPS settings

Set these in production when the application serves over HTTPS:

```python
SECURE_SSL_REDIRECT = True          # redirect all HTTP to HTTPS
SECURE_HSTS_SECONDS = 31536000      # 1 year HSTS header
SECURE_HSTS_INCLUDE_SUBDOMAINS = True
SECURE_HSTS_PRELOAD = True          # only after confirming production-readiness
SESSION_COOKIE_SECURE = True        # cookies only over HTTPS
CSRF_COOKIE_SECURE = True
SECURE_PROXY_SSL_HEADER = ("HTTP_X_FORWARDED_PROTO", "https")  # if behind a proxy
```

`SECURE_HSTS_PRELOAD = True` submits the site to the HSTS preload list. This is irreversible for the lifetime of the preload entry. Only enable after the site is permanently HTTPS.

## django-environ

`django-environ` (v0.14.0, actively maintained) reads environment variables into typed Python values and parses database URLs:

```python
import environ

env = environ.Env(DEBUG=(bool, False))
environ.Env.read_env(".env")

DEBUG = env("DEBUG")
DATABASES = {"default": env.db()}
```

Alternatives: `python-decouple`, raw `os.environ`. The pattern matters more than the library.

## References

- https://docs.djangoproject.com/en/stable/topics/security/
- https://docs.djangoproject.com/en/stable/ref/settings/#secure-ssl-redirect
- https://docs.djangoproject.com/en/stable/ref/settings/#std-setting-ALLOWED_HOSTS
- https://github.com/joke2k/django-environ
