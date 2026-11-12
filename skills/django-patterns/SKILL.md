---
name: django-patterns
description: Django patterns for the ORM, views, migrations, and settings. Load before any edit in a Django app (models, views, migrations, settings), however small, and for Django questions.
---

# Django patterns

Default assumption: Django 5.2 LTS.

- 6.1 is current stable, not LTS.
- 5.2 LTS has extended support through April 2028.
- Adapt advice to the version in the project's `requirements.txt`, `pyproject.toml`, or lockfile.
- DRF-specific patterns load when the project has `djangorestframework` in its dependencies.

## Reference files

Loading this skill isn't enough. Before your next edit, read the file of every row that matches it. The rules live in those files, and this one only points at them.

| Before                                                                             | Read                                               |
| ---------------------------------------------------------------------------------- | -------------------------------------------------- |
| Defining a model, its fields, constraints, indexes, or a manager                   | [models.md](reference/models.md)                   |
| Writing a queryset, or using `F()`, `Q()`, `prefetch_related`, or a bulk operation | [orm.md](reference/orm.md)                         |
| Writing a view or a form                                                           | [views-and-forms.md](reference/views-and-forms.md) |
| Writing or changing a template                                                     | [templates.md](reference/templates.md)             |
| Changing settings, secrets, or security headers                                    | [settings.md](reference/settings.md)               |
| Writing or changing a test                                                         | [testing.md](reference/testing.md)                 |
| Writing or squashing a migration                                                   | [migrations.md](reference/migrations.md)           |
| Registering or changing a `ModelAdmin`                                             | [admin.md](reference/admin.md)                     |
| Writing, changing, or reviewing Django code                                        | [anti-patterns.md](reference/anti-patterns.md)     |
