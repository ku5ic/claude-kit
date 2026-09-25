# Anti-patterns

Severity: `failure` = do not ship. `warning` = smell that compounds. `info` = hardening opportunity.

---

**`failure`: `DEBUG = True` in production**

Exposes full tracebacks, local variable values, SQL query history, and settings to any exception response. Confirmed security risk in Django security docs.

---

**`failure`: `|safe` filter on user-controlled content**

Bypasses auto-escaping. A documented XSS vector. If you see `{{ user_input|safe }}` or `{% autoescape off %}` wrapping user data, it will execute arbitrary HTML and JavaScript.

---

**`failure`: N+1 query in a template loop**

A database query inside a template loop (e.g., via a template tag that calls the ORM) scales with result count and cannot be caught statically. Measure with django-debug-toolbar. Fix with `select_related` or `prefetch_related` at the view or queryset level.

---

**`warning`: Business logic in `model.save()` override**

`QuerySet.update()` and `bulk_create()` / `bulk_update()` bypass `save()` and post-save signals. Logic in `save()` creates a false sense of coverage. Move to a service function that is called explicitly.

---

**`warning`: Signals for business logic**

Django signals decouple sender from receiver, which also hides the execution path from a code reader. Using signals for business logic creates invisible control flow that is hard to test, trace, and order. Signals are appropriate for cross-cutting concerns (audit logging, cache invalidation) where decoupling is the explicit goal. For business logic, call the function directly.

---

**`warning`: Fat views over ~200 lines**

A view function or method exceeding ~200 lines typically contains business logic that belongs in a service, manager method, or form validator. The threshold is judgment; the signal is a view that is hard to test in isolation.

---

**`warning`: `request.user` access inside model methods**

Models should not know about HTTP. Passing the request or `request.user` into a model method couples the model to the request lifecycle and makes it harder to test and reuse in management commands, async tasks, and admin actions.

---

**`warning`: `get_queryset()` doing auth checks**

Authorization belongs in permissions or explicit queryset filtering in the view, not inside `get_queryset()`. A queryset filtered by user inside `get_queryset()` is invisible to callers using `Model.objects.all()` and breaks admin access patterns.

---

**`info`: Bare `admin.site.register(Model)` without ModelAdmin**

Default admin registration gives a minimal interface. Explicit `ModelAdmin` with `list_display`, `search_fields`, and `list_select_related` is necessary for usable admin under incident conditions.
