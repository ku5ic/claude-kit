---
name: django-patterns
description: Django patterns - ORM gotchas, view design, migrations, settings, anti-patterns, and review checklist for Django backend work. Use whenever the project contains `manage.py`, `settings.py`, `apps.py`, or `django` in a Python manifest, OR the user asks about Django, its ORM, or any part of its request/model/template stack, even if Django is not mentioned by name.
---

# Django patterns

Django 5.2 LTS (extended support through April 2028). Django 6.1 is current stable (non-LTS). Baseline for this skill is 5.2 LTS. Adapt advice to the version in the project's `requirements.txt`, `pyproject.toml`, or lockfile.

DRF-specific patterns load when the project includes `djangorestframework` in dependencies.

## Reference files

| File                                               | Covers                                                                                      |
| -------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| [models.md](reference/models.md)                   | Model definition, Meta.constraints, Meta.indexes, null vs blank, on_delete, custom managers |
| [orm.md](reference/orm.md)                         | select_related, prefetch_related, F(), Q(), .only()/.defer(), .iterator(), bulk operations  |
| [views-and-forms.md](reference/views-and-forms.md) | CBV vs FBV, thin views, get_object_or_404, ModelForm, clean(), redirect after POST          |
| [templates.md](reference/templates.md)             | Template inheritance, auto-escaping, \|safe risks, {% csrf_token %}                         |
| [settings.md](reference/settings.md)               | Split settings, DEBUG, ALLOWED*HOSTS, SECRET_KEY, SECURE*\* family, django-environ          |
| [testing.md](reference/testing.md)                 | pytest-django, @pytest.mark.django_db, Factory Boy, model_bakery, --reuse-db                |
| [migrations.md](reference/migrations.md)           | Naming, atomic behavior, RunPython, makemigrations --check, squashing                       |
| [admin.md](reference/admin.md)                     | ModelAdmin, list_display, list_filter, search_fields, permissions, production posture       |
| [anti-patterns.md](reference/anti-patterns.md)     | Severity-labeled anti-patterns to flag in review                                            |

## References

- https://docs.djangoproject.com/en/stable/
- https://www.djangoproject.com/download/ (LTS and stable versions)
- https://pytest-django.readthedocs.io/en/latest/
- https://factoryboy.readthedocs.io/en/stable/
- https://github.com/joke2k/django-environ

## Version notes

Checked: 2026-09-12 against https://www.djangoproject.com/download/ and PyPI

- Django 6.1.1 current stable; 5.2 LTS supported to April 2028 (baseline here).
- pytest-django 4.14.0 (2026-08-10), django-environ 0.14.0 (2026-06-18), Factory Boy 3.3.3 (2025-02-03): pins in the reference files match these.
