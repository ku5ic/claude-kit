# Async correctness

| Declaration | Runs                                        | Use when                                                         |
| ----------- | ------------------------------------------- | ---------------------------------------------------------------- |
| `async def` | Awaited directly on the event loop          | Calling async libraries (httpx, asyncpg, aiofiles)               |
| `def`       | Run in an external threadpool, then awaited | Calling blocking libraries (requests, psycopg2, SQLAlchemy sync) |

- Dependencies follow the same split: a `def` dependency runs in the threadpool, an `async def` dependency is awaited directly.
- A blocking call, such as a sync ORM query, inside an `async def` route: make the route `def`, or switch to an async ORM (SQLAlchemy async, Tortoise ORM, databases).
- `async def` and `def` routes and dependencies mix freely in one app. Don't default to `async def`; a route that only calls sync libraries is `def`.
- CPU-bound work (image processing, PDF generation, data crunching): neither `def` nor `async def`. Offload it to a `concurrent.futures.ProcessPoolExecutor` or a task queue (Celery, ARQ).
