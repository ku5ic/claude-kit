# ViewSets and Routers

## ModelViewSet vs GenericViewSet + mixins

`ModelViewSet` provides all six standard actions: `.list()`, `.retrieve()`, `.create()`, `.update()`, `.partial_update()`, `.destroy()`. Use it when you want all six without customization.

`GenericViewSet` provides the base queryset and object retrieval behavior but no actions. Compose with mixins for only what you need:

```python
from rest_framework import mixins, viewsets

class OrderViewSet(
    mixins.CreateModelMixin,
    mixins.ListModelMixin,
    mixins.RetrieveModelMixin,
    viewsets.GenericViewSet,
):
    queryset = Order.objects.all()
    serializer_class = OrderSerializer
    permission_classes = [IsAuthenticated]
```

Avoid `ModelViewSet` when only a subset of actions are safe to expose. Explicitly composing mixins makes the surface area clear.

## Custom actions

`@action` routes additional methods as endpoints. `detail=True` adds a `pk` to the URL (operates on a single object). `detail=False` operates on the collection.

```python
from rest_framework.decorators import action
from rest_framework.response import Response

class OrderViewSet(viewsets.ModelViewSet):
    @action(detail=True, methods=["post"], permission_classes=[IsAuthenticated])
    def cancel(self, request, pk=None):
        order = self.get_object()
        order.cancel()
        return Response({"status": "cancelled"})
```

Custom actions can override `permission_classes` and `serializer_class` per-action.

## DefaultRouter

Register viewsets with a router rather than writing URL patterns manually:

```python
from rest_framework.routers import DefaultRouter

router = DefaultRouter()
router.register("orders", OrderViewSet)
urlpatterns = router.urls
```

`DefaultRouter` also creates a browsable API root at `/api/`. In production, consider whether exposing the API root to unauthenticated users is appropriate, or override `DEFAULT_RENDERER_CLASSES` to exclude `BrowsableAPIRenderer`.

## get_queryset and get_serializer_class

Override these methods to vary queryset or serializer by user or action:

```python
def get_queryset(self):
    return Order.objects.filter(user=self.request.user)

def get_serializer_class(self):
    if self.action in ("create", "update", "partial_update"):
        return OrderWriteSerializer
    return OrderReadSerializer
```

## References

- https://www.django-rest-framework.org/api-guide/viewsets/
- https://www.django-rest-framework.org/api-guide/routers/
- https://www.django-rest-framework.org/api-guide/generic-views/
