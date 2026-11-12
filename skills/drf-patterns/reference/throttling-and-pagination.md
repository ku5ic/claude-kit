# Throttling and Pagination

## Throttling

- `UserRateThrottle` limits requests per authenticated user and falls back to IP for anonymous ones. `AnonRateThrottle` limits unauthenticated requests by IP.
- Enable them in `DEFAULT_THROTTLE_CLASSES` and set rates in `DEFAULT_THROTTLE_RATES`, as in `{"anon": "100/day", "user": "1000/day"}`.
- The rate format is `<number>/<period>`, with periods `s` (second), `m` (minute), `h` (hour), and `d` (day).
- Throttle authentication and token endpoints harder than general read endpoints. `ScopedRateThrottle` gives each API section its own rate, as in `throttle_scope = "token_obtain"` on the view with `"token_obtain": "5/minute"` in `DEFAULT_THROTTLE_RATES`.
- For per-endpoint control, override `throttle_classes` on a viewset or an `@action`.

## Pagination

- `DEFAULT_PAGINATION_CLASS` and `PAGE_SIZE` default to `None`, and an unconfigured list endpoint returns every row. Set a global default, such as `"rest_framework.pagination.PageNumberPagination"` with `"PAGE_SIZE": 20`.
- `PageNumberPagination` takes `?page=2` and returns `count`, `next`, `previous`, and `results`.
- `LimitOffsetPagination` takes `?limit=20&offset=40`. It's more flexible for clients and slightly more expensive at high offsets.
- `CursorPagination` uses an opaque cursor with no random access. Use it for large datasets with frequent inserts; it needs a stable, unique ordering, typically `created_at`.
- When one resource needs a different style, override `pagination_class` on its viewset.
