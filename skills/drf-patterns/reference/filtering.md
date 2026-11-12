# Filtering

## django-filter

- Field-based filtering: install `django-filter`, add `"django_filters.rest_framework.DjangoFilterBackend"` to `DEFAULT_FILTER_BACKENDS`, and set `filterset_fields = ["category", "is_active"]` on the viewset.
- Complex filtering: a `django_filters.FilterSet` class, set as the viewset's `filterset_class`, with filters such as `min_price = django_filters.NumberFilter(field_name="price", lookup_expr="gte")`.

## SearchFilter and OrderingFilter

- `SearchFilter` enables `?search=<term>` across the declared `search_fields`. The default lookup is `icontains`.
- `OrderingFilter` enables `?ordering=price` or `?ordering=-price`. Always declare `ordering_fields` explicitly; `ordering` sets the default order.

## get_queryset override

- User-scoped or context-sensitive filtering: override `get_queryset()`, as in `return Order.objects.filter(user=self.request.user)`.

## Never do this

- `Product.objects.filter(**self.request.query_params.dict())` passes raw client input as ORM keyword arguments: `failure`. Use `django-filter` or an explicit `get_queryset()` override.
