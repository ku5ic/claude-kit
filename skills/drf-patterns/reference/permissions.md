# Permissions

## Always set explicit permission_classes

- DRF's default permission policy is `AllowAny`, unrestricted access regardless of authentication. Set `permission_classes` explicitly on every view or viewset, as in `permission_classes = [IsAuthenticated]`, and don't rely on the global default.
- Or set a safe global default, `"DEFAULT_PERMISSION_CLASSES": ["rest_framework.permissions.IsAuthenticated"]` in `REST_FRAMEWORK`, and override it only where looser access is required.

## IsAuthenticated vs IsAuthenticatedOrReadOnly

- A resource with write semantics: `IsAuthenticated`, which requires authentication for every HTTP method.
- Public reads with authenticated writes, such as a public product catalog where reviews require login: `IsAuthenticatedOrReadOnly`. GET, HEAD, and OPTIONS stay open; POST, PUT, PATCH, and DELETE require authentication.

## Object-level permissions

- Per-instance authorization: implement `has_object_permission(self, request, view, obj)`. Generic views call it through `.get_object()`; a custom view must call `check_object_permissions(request, obj)` itself.
- `has_permission` (view-level) runs before `has_object_permission`, and both must pass.
- An owner-or-read-only check returns `True` when `request.method in permissions.SAFE_METHODS` and `obj.owner == request.user` otherwise.

## Composing permissions

- Several classes in `permission_classes` are ANDed. For other logic, `BasePermission` subclasses compose with `&` (and), `|` (or), and `~` (not), as in `permission_classes = [IsAuthenticated | ReadOnly]`.
