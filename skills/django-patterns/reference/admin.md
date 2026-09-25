# Admin

## ModelAdmin basics

Register every user-facing model with an explicit `ModelAdmin`. The bare `admin.site.register(Model)` default is only acceptable for developer-only debug models.

```python
@admin.register(Product)
class ProductAdmin(admin.ModelAdmin):
    list_display = ["name", "price", "is_active", "created_at"]
    list_filter = ["is_active", "category"]
    search_fields = ["name", "sku"]
    readonly_fields = ["created_at", "updated_at"]
    list_select_related = ["category"]
```

`list_display`: fields shown on the changelist. Keep it to the columns actually useful in operations. Too many columns degrades performance and readability.

`list_filter`: sidebar filter. For large tables, `show_facets = admin.ShowFacets.NEVER` avoids expensive COUNT queries per filter.

`search_fields`: full-text search. Default lookup is `icontains`. Limit to indexed fields on large tables.

`list_select_related`: prevents N+1 queries when displaying related object fields. Set to a list of field names, not `True` (which follows all relations).

## readonly_fields

Computed fields, timestamps, and any field you do not want changed through admin belong in `readonly_fields`. They must also appear in `fields` or `fieldsets` to display.

## Permissions

The permission methods (`has_add_permission`, `has_change_permission`, `has_delete_permission`, `has_view_permission`) mirror the application's own authorization rules. Do not rely on admin to enforce what the application does not.

Filter querysets for non-superusers explicitly:

```python
def get_queryset(self, request):
    qs = super().get_queryset(request)
    if not request.user.is_superuser:
        return qs.filter(owner=request.user)
    return qs
```

## Production posture

Admin is useful for incident investigation. Keep it simple and readable. Do not use it as a workflow UI for business processes; that is what the application is for.

`show_full_result_count = False` on large tables avoids the expensive `COUNT(*)` query that powers the "X results" indicator.

## References

- https://docs.djangoproject.com/en/stable/ref/contrib/admin/
- https://docs.djangoproject.com/en/stable/ref/contrib/admin/#modeladmin-options
