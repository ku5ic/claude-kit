# Anti-patterns

Severity levels: the report-format skill.

- A write endpoint without `permission_classes`: `failure`. See `reference/permissions.md`.
- `queryset.filter(**request.query_params.dict())`: `failure`. Use `django-filter` or an explicit `get_queryset()` override.
- A list endpoint without pagination: `failure`. Set `DEFAULT_PAGINATION_CLASS` and `PAGE_SIZE` in settings.
- `fields = "__all__"` on a serializer: `warning`. List the fields explicitly.
- Business logic in serializer `validate()`: `warning`. Keep `validate()` to data integrity constraints (dates in order, quantities positive); authorization checks and side effects go in the view or a service layer.
- An authentication endpoint without throttling: `warning`. Apply `ScopedRateThrottle` with a tight rate, such as `5/minute`, to any endpoint that issues or validates credentials.
- `OrderingFilter` without `ordering_fields`: `warning`. Declare `ordering_fields` explicitly.
- `BrowsableAPIRenderer` in production: `info`. Unless the API explorer is intended, remove it from `DEFAULT_RENDERER_CLASSES` in production settings.
