# Admin

## ModelAdmin

- Every user-facing model: an explicit `ModelAdmin` via `@admin.register(Product)`. A bare `admin.site.register(Model)` is only acceptable for developer-only debug models.
- `list_display`: only the columns that are useful in operations.
- `list_select_related`: a list of the related field names the changelist shows, so it doesn't run N+1 queries. `True` is deprecated since Django 6.1.
- `search_fields`: the default lookup is `icontains`. On a large table, list only indexed fields.
- A large table: `show_facets = admin.ShowFacets.NEVER` skips the COUNT query per filter, and `show_full_result_count = False` skips the `COUNT(*)` behind the "X results" indicator.
- Computed fields, timestamps, and fields admin mustn't change: `readonly_fields`. They also need to appear in `fields` or `fieldsets` to display.

## Permissions

- `has_add_permission`, `has_change_permission`, `has_delete_permission`, and `has_view_permission` mirror the application's own authorization rules. Never rely on admin to enforce what the application doesn't.
- Non-superusers: filter in `get_queryset()` explicitly, such as `qs.filter(owner=request.user)` when `request.user.is_superuser` is false.

## Production use

- Admin is for incident investigation. Keep it simple and readable, and never use it as a workflow UI for business processes.
