---
name: drf-patterns
description: Django REST Framework patterns for serializers, viewsets, permissions, and throttling. Load before any edit to a serializer, viewset, or API view, however small, and for DRF questions.
---

# Django REST Framework patterns

DRF 3.18.x.

- 3.18 supports Django 5.2, 6.0, and 6.1. 3.18.0 dropped Django 4.2, 5.0, and 5.1.
- Adapt advice to the version in the project's `requirements.txt`, `pyproject.toml`, or lockfile.

## Reference files

Loading this skill isn't enough. Before your next edit, read the file of every row that matches it. The rules live in those files, and this one only points at them.

| Before                                                               | Read                                                                   |
| -------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| Writing or changing a serializer, its fields, validation, or nesting | [serializers.md](reference/serializers.md)                             |
| Writing a viewset, a router, an `@action`, or `get_queryset`         | [viewsets-and-routers.md](reference/viewsets-and-routers.md)           |
| Setting or changing `permission_classes`                             | [permissions.md](reference/permissions.md)                             |
| Adding throttling or pagination                                      | [throttling-and-pagination.md](reference/throttling-and-pagination.md) |
| Adding filtering, search, or ordering                                | [filtering.md](reference/filtering.md)                                 |
| Setting up authentication, tokens, or JWT                            | [auth.md](reference/auth.md)                                           |
| Versioning the API                                                   | [versioning.md](reference/versioning.md)                               |
| Writing, changing, or reviewing DRF code                             | [anti-patterns.md](reference/anti-patterns.md)                         |
