---
name: django-patterns
description: Django patterns for the ORM, views, migrations, and settings. Use for manage.py, settings.py, apps.py, django in a Python manifest, or Django questions.
---

# Django patterns

Default assumption: Django 5.2 LTS.

- 6.1 is current stable, not LTS.
- 5.2 LTS has extended support through April 2028.
- Adapt advice to the version in the project's `requirements.txt`, `pyproject.toml`, or lockfile.
- DRF-specific patterns load when the project has `djangorestframework` in its dependencies.

## Reference files

| File                                               | Covers                                                                                     |
| -------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| [models.md](reference/models.md)                   | null vs blank, on_delete, related_name, Meta.constraints, Meta.indexes, managers           |
| [orm.md](reference/orm.md)                         | select_related, prefetch_related, F(), Q(), .only()/.defer(), .iterator(), bulk operations |
| [views-and-forms.md](reference/views-and-forms.md) | CBV vs FBV, thin views, get_object_or_404, ModelForm, clean(), redirect after POST         |
| [templates.md](reference/templates.md)             | Template inheritance, auto-escaping, \|safe risks, {% csrf_token %}, @csrf_exempt          |
| [settings.md](reference/settings.md)               | Split settings, DEBUG, ALLOWED_HOSTS, SECRET_KEY, SECURE\_\* family, django-environ        |
| [testing.md](reference/testing.md)                 | pytest-django, @pytest.mark.django_db, Factory Boy, model_bakery, --reuse-db               |
| [migrations.md](reference/migrations.md)           | Naming, atomic behavior, RunPython, makemigrations --check, squashing                      |
| [admin.md](reference/admin.md)                     | ModelAdmin, list_display, list_select_related, search_fields, permissions, production use  |
| [anti-patterns.md](reference/anti-patterns.md)     | Nine anti-patterns with severity calls                                                     |
