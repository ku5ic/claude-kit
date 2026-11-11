---
name: drf-patterns
description: Django REST Framework patterns for serializers, viewsets, permissions, and throttling. Use for djangorestframework or rest_framework in a project, or DRF questions.
---

# Django REST Framework patterns

DRF 3.18.x.

- 3.18 supports Django 5.2, 6.0, and 6.1. 3.18.0 dropped Django 4.2, 5.0, and 5.1.
- Adapt advice to the version in the project's `requirements.txt`, `pyproject.toml`, or lockfile.

## Reference files

Read every file in this table as soon as this skill loads. A hook blocks every tool call until you have.

| Before                                                               | Read                                                                   |
| -------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| Writing or changing a serializer, its fields, validation, or nesting | [serializers.md](reference/serializers.md)                             |
| Writing a viewset, a router, an `@action`, or `get_queryset`         | [viewsets-and-routers.md](reference/viewsets-and-routers.md)           |
| Setting or changing `permission_classes`                             | [permissions.md](reference/permissions.md)                             |
| Adding throttling or pagination                                      | [throttling-and-pagination.md](reference/throttling-and-pagination.md) |
| Adding filtering, search, or ordering                                | [filtering.md](reference/filtering.md)                                 |
| Setting up authentication, tokens, or JWT                            | [auth.md](reference/auth.md)                                           |
| Versioning the API                                                   | [versioning.md](reference/versioning.md)                               |
| Reviewing DRF code                                                   | [anti-patterns.md](reference/anti-patterns.md)                         |
