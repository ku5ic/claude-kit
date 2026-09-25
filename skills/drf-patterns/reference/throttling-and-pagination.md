# Throttling and Pagination

## Throttling

`UserRateThrottle` limits requests per authenticated user (falls back to IP for anonymous). `AnonRateThrottle` limits unauthenticated requests by IP.

```python
REST_FRAMEWORK = {
    "DEFAULT_THROTTLE_CLASSES": [
        "rest_framework.throttling.AnonRateThrottle",
        "rest_framework.throttling.UserRateThrottle",
    ],
    "DEFAULT_THROTTLE_RATES": {
        "anon": "100/day",
        "user": "1000/day",
    },
}
```

Rate format: `<number>/<period>`. Periods: `s` (second), `m` (minute), `h` (hour), `d` (day).

Throttle authentication and token endpoints more aggressively than general read endpoints. Credential stuffing targets login and token endpoints specifically. `ScopedRateThrottle` lets you assign separate rate limits per API section:

```python
class ObtainTokenView(APIView):
    throttle_scope = "token_obtain"

REST_FRAMEWORK = {
    "DEFAULT_THROTTLE_RATES": {
        "token_obtain": "5/minute",
    }
}
```

Override `throttle_classes` on individual viewsets or `@action` decorators for per-endpoint control.

## Pagination

`DEFAULT_PAGINATION_CLASS` and `PAGE_SIZE` default to `None` in DRF: no pagination until you configure it. An unconfigured list endpoint returns all rows, which will become a problem at scale.

Set a global default:

```python
REST_FRAMEWORK = {
    "DEFAULT_PAGINATION_CLASS": "rest_framework.pagination.PageNumberPagination",
    "PAGE_SIZE": 20,
}
```

Three built-in styles:

`PageNumberPagination`: `?page=2`. Simple. Returns `count`, `next`, `previous`, `results`.

`LimitOffsetPagination`: `?limit=20&offset=40`. More flexible for clients. Slightly more expensive at high offsets.

`CursorPagination`: opaque cursor, no random access. Best for large datasets with frequent inserts (prevents duplicate items from pagination drift). Requires a stable, unique ordering (typically `created_at`).

Override `pagination_class` on individual viewsets when a different style is needed for that resource.

## References

- https://www.django-rest-framework.org/api-guide/throttling/
- https://www.django-rest-framework.org/api-guide/pagination/
