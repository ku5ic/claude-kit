# django-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- https://docs.djangoproject.com/en/stable/
- https://www.djangoproject.com/download/ (LTS and stable versions)
- https://pytest-django.readthedocs.io/en/latest/
- https://factoryboy.readthedocs.io/en/stable/
- https://github.com/joke2k/django-environ

### Version notes

Checked: 2026-11-10 against https://www.djangoproject.com/download/, https://docs.djangoproject.com/en/stable/ref/settings/, https://docs.djangoproject.com/en/stable/ref/contrib/admin/, and PyPI

- Django 6.1.3 current stable; 5.2 LTS (5.2.19) supported to April 2028 (baseline here). 6.1 deprecates `list_select_related = True`.
- pytest-django 4.14.0 (2026-08-10), django-environ 0.14.0 (2026-06-18), Factory Boy 3.3.3 (2025-02-03): pins in the reference files match these.

### Left to linters

Checked: 2026-11-11 with ruff 0.17.4 defaults (`uvx ruff check --isolated`, no config, no `--select`) on a probe package. The default set has no `DJ` rules, and from `S` only `S102`, `S110`, and `S112`. This rule was dropped because a default check reports it; don't add it back.

- ruff `invalid-syntax` reports a `Q()` placed after a keyword argument or after `**kwargs` in `filter()` or `get()`. Python rejects it at compile time as a `SyntaxError`.
- ruff `B026` reports Q objects unpacked with `*args` after a keyword argument.

Not caught by defaults, so kept: a hardcoded `SECRET_KEY` (`S105` is off), `DEBUG = True`, `mark_safe()` (`S308` is off), `@csrf_exempt`, `null=True` on a `CharField` (`DJ001` is off), a bare `admin.site.register(Model)`, `obj.field += 1; obj.save()`, a manual `DoesNotExist` catch, and logic in a `save()` override.

## reference/admin.md

### References

- https://docs.djangoproject.com/en/stable/ref/contrib/admin/
- https://docs.djangoproject.com/en/stable/ref/contrib/admin/#modeladmin-options

## reference/migrations.md

### References

- https://docs.djangoproject.com/en/stable/topics/migrations/
- https://docs.djangoproject.com/en/stable/ref/django-admin/#makemigrations
- https://docs.djangoproject.com/en/stable/howto/writing-migrations/

## reference/models.md

### References

- https://docs.djangoproject.com/en/stable/topics/db/models/
- https://docs.djangoproject.com/en/stable/ref/models/fields/
- https://docs.djangoproject.com/en/stable/ref/models/options/

## reference/orm.md

### References

- https://docs.djangoproject.com/en/stable/ref/models/querysets/#select-related
- https://docs.djangoproject.com/en/stable/ref/models/querysets/#prefetch-related
- https://docs.djangoproject.com/en/stable/ref/models/expressions/#f-expressions
- https://docs.djangoproject.com/en/stable/topics/db/queries/#complex-lookups-with-q-objects
- https://docs.djangoproject.com/en/stable/ref/models/querysets/#iterator
- https://docs.djangoproject.com/en/stable/ref/models/querysets/#bulk-create

## reference/settings.md

### References

- https://docs.djangoproject.com/en/stable/topics/security/
- https://docs.djangoproject.com/en/stable/ref/settings/#secure-ssl-redirect
- https://docs.djangoproject.com/en/stable/ref/settings/#std-setting-ALLOWED_HOSTS
- https://github.com/joke2k/django-environ

## reference/templates.md

### References

- https://docs.djangoproject.com/en/stable/topics/templates/
- https://docs.djangoproject.com/en/stable/topics/security/ (XSS via |safe)
- https://docs.djangoproject.com/en/stable/ref/csrf/

## reference/testing.md

### References

- https://pytest-django.readthedocs.io/en/latest/
- https://factoryboy.readthedocs.io/en/stable/
- https://model-bakery.readthedocs.io/en/latest/

## reference/views-and-forms.md

### References

- https://docs.djangoproject.com/en/stable/topics/class-based-views/
- https://docs.djangoproject.com/en/stable/topics/forms/
- https://docs.djangoproject.com/en/stable/ref/class-based-views/
- https://docs.djangoproject.com/en/stable/topics/forms/modelforms/
