# ViewSets and Routers

## ModelViewSet vs GenericViewSet + mixins

- `ModelViewSet` provides all six standard actions: `.list()`, `.retrieve()`, `.create()`, `.update()`, `.partial_update()`, `.destroy()`. Use it only when all six are safe to expose without customization.
- Otherwise, compose `GenericViewSet`, which has queryset and object retrieval but no actions, with only the mixins you need, such as `mixins.CreateModelMixin`, `mixins.ListModelMixin`, and `mixins.RetrieveModelMixin`.

## Custom actions

- An extra endpoint on a viewset: `@action`. `detail=True` acts on one object and adds a `pk` to the URL; `detail=False` acts on the collection.
- An `@action` can override `permission_classes` and `serializer_class` for itself, as in `@action(detail=True, methods=["post"], permission_classes=[IsAuthenticated])`.

## DefaultRouter

- Register viewsets with a router (`router = DefaultRouter()`, `router.register("orders", OrderViewSet)`, `urlpatterns = router.urls`), not hand-written URL patterns.
- `DefaultRouter` also creates a browsable API root at `/api/`. In production, decide whether unauthenticated users should reach it, or exclude `BrowsableAPIRenderer` from `DEFAULT_RENDERER_CLASSES`.

## get_queryset and get_serializer_class

- A queryset or serializer that varies by user or action: override `get_queryset()` or `get_serializer_class()`, as in returning `OrderWriteSerializer` when `self.action in ("create", "update", "partial_update")`.
