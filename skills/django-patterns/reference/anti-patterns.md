# Anti-patterns

Severity levels: the report-format skill.

- `DEBUG = True` in production: `failure`. See `reference/settings.md`.
- `|safe`, `{% autoescape off %}`, or `mark_safe()` on user-controlled content: `failure`. See `reference/templates.md`.
- A database query inside a template loop, such as a template tag that calls the ORM: `failure`. No static check catches it, so measure with django-debug-toolbar. Fix it with `select_related` or `prefetch_related` at the view or queryset level.
- Business logic in a `model.save()` override: `warning`. `QuerySet.update()`, `bulk_create()`, and `bulk_update()` skip `save()` and post-save signals. Move the logic to a service function that's called explicitly.
- Signals for business logic: `warning`. Call the function directly. Signals fit cross-cutting concerns such as audit logging and cache invalidation, where decoupling is the explicit goal.
- A view past the function-size threshold in `skills/engineering-fundamentals/reference/metric-thresholds.md`: `warning`. Move its business logic to a service, manager method, or form validator.
- The request or `request.user` passed into a model method: `warning`. Models don't know about HTTP.
- Auth checks inside a model manager's `get_queryset()`: `warning`. Authorization goes in permissions or explicit queryset filtering in the view.
- A bare `admin.site.register(Model)` without a `ModelAdmin`: `info`. Add one with `list_display`, `search_fields`, and `list_select_related`.
