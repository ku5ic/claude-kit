# Filtering

## django-filter

The recommended approach for field-based filtering. Install `django-filter` and configure `DjangoFilterBackend`:

```python
REST_FRAMEWORK = {
    "DEFAULT_FILTER_BACKENDS": ["django_filters.rest_framework.DjangoFilterBackend"],
}

class ProductViewSet(viewsets.ModelViewSet):
    filterset_fields = ["category", "is_active"]
```

For complex filtering, define a `FilterSet` class:

```python
import django_filters

class ProductFilter(django_filters.FilterSet):
    min_price = django_filters.NumberFilter(field_name="price", lookup_expr="gte")
    max_price = django_filters.NumberFilter(field_name="price", lookup_expr="lte")

    class Meta:
        model = Product
        fields = ["category", "is_active"]

class ProductViewSet(viewsets.ModelViewSet):
    filterset_class = ProductFilter
```

## SearchFilter and OrderingFilter

`SearchFilter` enables `?search=<term>` across declared `search_fields`. Default lookup is `icontains`.

`OrderingFilter` enables `?ordering=price` or `?ordering=-price`. Always declare `ordering_fields` explicitly. The DRF docs warn that unrestricted ordering can expose sensitive field names or allow ordering by password hash fields.

```python
class ProductViewSet(viewsets.ModelViewSet):
    filter_backends = [SearchFilter, OrderingFilter]
    search_fields = ["name", "description"]
    ordering_fields = ["price", "created_at"]
    ordering = ["name"]  # default ordering
```

## get_queryset override

For user-scoped or context-sensitive filtering, override `get_queryset()`:

```python
def get_queryset(self):
    return Order.objects.filter(user=self.request.user)
```

## Never do this

`queryset.filter(**request.query_params)` passes raw client input directly as ORM keyword arguments. This exposes internal field names, allows clients to filter on any column including sensitive ones, and can surface data they should not see. Use `django-filter` or an explicit override.

```python
# WRONG - failure
def get_queryset(self):
    return Product.objects.filter(**self.request.query_params.dict())
```

## References

- https://www.django-rest-framework.org/api-guide/filtering/
- https://django-filter.readthedocs.io/en/stable/
