# Anti-patterns

Severity levels: the report-format skill.

---

**`failure`: Missing permission_classes on write endpoints**

See `reference/permissions.md`.

---

**`failure`: `queryset.filter(**request.query_params.dict())`\*\*

Passes raw client-controlled input as ORM keyword arguments. Clients can filter on any model field, including sensitive internal ones. Use `django-filter` or an explicit `get_queryset()` override.

---

**`failure`: No pagination on list endpoints**

DRF does not paginate by default. An endpoint returning an unbounded queryset will eventually cause memory and timeout issues at scale. Set `DEFAULT_PAGINATION_CLASS` and `PAGE_SIZE` in settings.

---

**`warning`: `fields = "__all__"` on a serializer**

DRF docs recommend explicit fields to prevent "unintentionally exposing data when your models change." New columns added to the model are automatically exposed. Enumerate fields explicitly.

---

**`warning`: Business logic in serializer `validate()`**

`validate()` is for data integrity constraints (dates in order, quantities positive). Authorization checks and side effects belong in the view or a service layer. `validate()` runs before the view has enforced permissions in some flow variants.

---

**`warning`: No throttling on authentication endpoints**

Token obtain/refresh endpoints without throttling are vulnerable to credential stuffing. Apply `ScopedRateThrottle` with a tight rate (e.g., `5/minute`) on any endpoint that issues or validates credentials.

---

**`warning`: OrderingFilter without explicit `ordering_fields`**

DRF docs warn that unrestricted ordering can expose sensitive field names. Always declare `ordering_fields = ["safe_field_1", "safe_field_2"]`.

---

**`info`: `BrowsableAPIRenderer` in production**

`DefaultRouter` and `BrowsableAPIRenderer` expose a clickable API explorer. Not a security vulnerability by itself, but it provides a map of your API to unauthenticated visitors. Remove from `DEFAULT_RENDERER_CLASSES` in production settings if this is not intentional.
