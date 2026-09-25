# Dependency Injection

## Depends() basics

`Depends()` declares that a route parameter comes from a dependency function, not the request body or path.

```python
from typing import Annotated
from fastapi import Depends, FastAPI

app = FastAPI()

def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()

def get_current_user(token: str = Depends(oauth2_scheme)) -> User:
    ...

@app.get("/orders/")
def list_orders(
    db: Annotated[Session, Depends(get_db)],
    user: Annotated[User, Depends(get_current_user)],
):
    return db.query(Order).filter(Order.user_id == user.id).all()
```

Prefer the `Annotated[T, Depends(fn)]` form over `param: T = Depends(fn)`. It keeps the type annotation and the injection separate, making the type readable without looking at the default.

## Yield dependencies

Use `yield` when the dependency requires cleanup after the response (database sessions, file handles, external connections):

```python
def get_db():
    db = SessionLocal()
    try:
        yield db        # db is available in the route
    finally:
        db.close()      # runs after the response is sent
```

Code after `yield` runs after the response is sent to the client. Exceptions in the route propagate to the finally block. Do not catch and swallow exceptions in yield dependencies unless you intend to change error behavior.

## Dependency classes

Classes are valid dependencies when they need configuration:

```python
class PaginationParams:
    def __init__(self, skip: int = 0, limit: int = 20):
        self.skip = skip
        self.limit = limit

@app.get("/items/")
def list_items(pagination: Annotated[PaginationParams, Depends()]):
    return db.query(Item).offset(pagination.skip).limit(pagination.limit).all()
```

`Depends()` with no argument on a class call injects the class itself as a dependency (calls `__init__` with query params).

## Dependency caching

FastAPI caches dependency results within a single request. If two route parameters depend on the same function, the function runs once and its result is shared. Use `Depends(fn, use_cache=False)` to force re-execution (rare; mainly useful when the dependency has side effects that should repeat).

## Testing overrides

Override dependencies in tests without modifying route code:

```python
def override_get_db():
    db = TestingSessionLocal()
    try:
        yield db
    finally:
        db.close()

app.dependency_overrides[get_db] = override_get_db

# Reset after tests to avoid leakage between test modules
app.dependency_overrides = {}
```

Always reset overrides after tests. Leaving overrides in place leaks test state across modules.

## References

- https://fastapi.tiangolo.com/tutorial/dependencies/
- https://fastapi.tiangolo.com/tutorial/dependencies/dependencies-with-yield/
- https://fastapi.tiangolo.com/advanced/testing-dependencies/
