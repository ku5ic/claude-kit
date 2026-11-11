# Permissions

## Always set explicit permission_classes

The default DRF permission policy is `AllowAny`: unrestricted access regardless of authentication. This is the right default for a framework (fail open for development), but the wrong posture for any production endpoint.

Set `permission_classes` explicitly on every viewset or view. Do not rely on the global default:

```python
class OrderViewSet(viewsets.ModelViewSet):
    permission_classes = [IsAuthenticated]
```

Or set a safe global default in settings and override where looser access is genuinely required:

```python
REST_FRAMEWORK = {
    "DEFAULT_PERMISSION_CLASSES": ["rest_framework.permissions.IsAuthenticated"],
}
```

## IsAuthenticated vs IsAuthenticatedOrReadOnly

`IsAuthenticated`: all HTTP methods require authentication. Use for any resource with write semantics.

`IsAuthenticatedOrReadOnly`: GET, HEAD, OPTIONS are open; POST, PUT, PATCH, DELETE require authentication. Use for public read + authenticated write patterns (e.g., a public product catalog where reviews require login).

```python
class ProductViewSet(viewsets.ModelViewSet):
    permission_classes = [IsAuthenticatedOrReadOnly]
```

## Object-level permissions

`has_object_permission(self, request, view, obj)` allows per-instance authorization. Generic views call it automatically via `.get_object()`. Custom views must call `check_object_permissions(request, obj)` explicitly.

```python
class IsOwnerOrReadOnly(permissions.BasePermission):
    def has_object_permission(self, request, view, obj):
        if request.method in permissions.SAFE_METHODS:
            return True
        return obj.owner == request.user
```

Note: `has_permission` (view-level) is called before `has_object_permission`. Both must pass.

## Composing permissions

Multiple classes in `permission_classes` are ANDed. For other logic, classes inheriting `BasePermission` compose with `&` (and), `|` (or), and `~` (not): `permission_classes = [IsAuthenticated | ReadOnly]`.
