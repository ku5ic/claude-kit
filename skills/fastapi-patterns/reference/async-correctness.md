# Async Correctness

## async def vs def

FastAPI supports both. The choice determines how the function runs:

| Declaration | Execution                                   | Use when                                                         |
| ----------- | ------------------------------------------- | ---------------------------------------------------------------- |
| `async def` | Awaited directly on the event loop          | calling async libraries (httpx, asyncpg, aiofiles)               |
| `def`       | Run in an external threadpool, then awaited | calling blocking libraries (requests, psycopg2, SQLAlchemy sync) |

```python
# Correct: blocking SQLAlchemy call in a def route
@app.get("/users/{user_id}")
def get_user(user_id: int, db: Annotated[Session, Depends(get_db)]):
    return db.query(User).filter(User.id == user_id).first()

# Correct: async httpx call in an async def route
@app.get("/external-data/")
async def fetch_external(client: Annotated[httpx.AsyncClient, Depends(get_client)]):
    response = await client.get("https://api.example.com/data")
    return response.json()
```

The same rule applies to dependencies: `def` dependencies run in the threadpool; `async def` dependencies are awaited directly.

## The blocking-in-async trap

Calling blocking I/O directly inside an `async def` route stalls the event loop for every concurrent request:

```python
# WRONG: blocks the event loop
@app.get("/users/{user_id}")
async def get_user(user_id: int, db: Annotated[Session, Depends(get_db)]):
    return db.query(User).filter(User.id == user_id).first()  # sync ORM in async route
```

Fix: either switch the route to `def`, or use an async ORM (SQLAlchemy async, Tortoise ORM, databases).

```python
# Correct: use def with sync ORM
@app.get("/users/{user_id}")
def get_user(user_id: int, db: Annotated[Session, Depends(get_db)]):
    return db.query(User).filter(User.id == user_id).first()
```

## Mixing sync and async code

You can freely mix `async def` and `def` routes and dependencies in the same app. FastAPI handles both correctly. Do not default to `async def` everywhere; use `def` for routes that only call synchronous libraries.

## CPU-bound work

Neither `def` nor `async def` is appropriate for CPU-bound work (image processing, PDF generation, data crunching). Both will occupy their execution context (threadpool or event loop) for the duration. Offload CPU-bound tasks to a process pool (`concurrent.futures.ProcessPoolExecutor`) or a task queue (Celery, ARQ).

## References

- https://fastapi.tiangolo.com/async/
