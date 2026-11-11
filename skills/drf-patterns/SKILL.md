---
name: drf-patterns
description: Django REST Framework patterns for serializers, viewsets, permissions, and throttling. Use for djangorestframework or rest_framework in a project, or DRF questions.
---

# Django REST Framework patterns

DRF 3.18.x.

- 3.18 supports Django 5.2, 6.0, and 6.1. 3.18.0 dropped Django 4.2, 5.0, and 5.1.
- Adapt advice to the version in the project's `requirements.txt`, `pyproject.toml`, or lockfile.

## Reference files

| File                                                                   | Covers                                                                                           |
| ---------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| [serializers.md](reference/serializers.md)                             | Explicit fields (3.5+), `validate()`, nested serializers, `SerializerMethodField`, read vs write |
| [viewsets-and-routers.md](reference/viewsets-and-routers.md)           | `ModelViewSet` vs `GenericViewSet` + mixins, `@action`, `DefaultRouter`, `get_queryset`          |
| [permissions.md](reference/permissions.md)                             | `AllowAny` default, `IsAuthenticated` vs `IsAuthenticatedOrReadOnly`, object-level, composition  |
| [throttling-and-pagination.md](reference/throttling-and-pagination.md) | User, anon, and scoped throttles, pagination styles, page size config                            |
| [filtering.md](reference/filtering.md)                                 | django-filter, `SearchFilter`, `OrderingFilter`, `get_queryset` override                         |
| [auth.md](reference/auth.md)                                           | Token, session, and basic auth, JWT via simplejwt                                                |
| [versioning.md](reference/versioning.md)                               | Five versioning schemes, `request.version`, `ALLOWED_VERSIONS`                                   |
| [anti-patterns.md](reference/anti-patterns.md)                         | Eight severity-labeled anti-patterns to flag in review                                           |
