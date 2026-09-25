# Anti-Patterns

## failure: blocking I/O in an async def route

```python
# WRONG
@app.get("/users/{user_id}")
async def get_user(user_id: int, db: Session = Depends(get_db)):
    return db.query(User).get(user_id)  # sync ORM call in async route
```

Stalls the event loop for every concurrent request. Fix: use `def` for routes that call synchronous libraries, or switch to an async ORM.

## failure: returning HTTPException instead of raising it

```python
# WRONG
@app.get("/items/{item_id}")
def get_item(item_id: int):
    if item_id < 0:
        return HTTPException(status_code=400, detail="Invalid id")
    ...
```

`return` produces a 200 response with the exception object serialized as JSON. Always `raise HTTPException(...)`.

## failure: no response_model on routes that return ORM objects

```python
# WRONG
@app.get("/users/{user_id}")
def get_user(user_id: int, db: Session = Depends(get_db)):
    return db.query(User).filter(User.id == user_id).first()
```

Without `response_model`, FastAPI serializes the entire ORM object, including columns that should stay internal (hashed_password, internal flags). Always declare `response_model=UserRead`.

## failure: leaking internal errors in detail

```python
# WRONG
except sqlalchemy.exc.IntegrityError as exc:
    raise HTTPException(status_code=400, detail=str(exc))
```

`str(exc)` on a SQLAlchemy IntegrityError contains the raw SQL, table name, and constraint name. Log the exception, return a generic message.

## warning: missing Depends() for auth on routes that need it

```python
# WRONG: forgetting the dependency
@app.delete("/users/{user_id}")
def delete_user(user_id: int, db: Session = Depends(get_db)):
    ...  # no get_current_user dependency
```

Routes without an auth dependency are publicly accessible. Every mutating endpoint (POST, PUT, PATCH, DELETE) should depend on an auth dependency unless the route is explicitly public.

## warning: using query parameters as API keys

```python
api_key_query = APIKeyQuery(name="api_key")
```

Query parameters appear in server logs, browser history, and referrer headers. Use `APIKeyHeader` instead.

## warning: leaving Depends() overrides in place after tests

```python
app.dependency_overrides[get_db] = override_get_db
# ... tests ...
# missing: app.dependency_overrides = {}
```

Overrides leak across test modules. Always reset after the test or test module that sets them.

## warning: using Pydantic v1 patterns in a v2 project

```python
# v1 patterns that fail or behave differently in v2
class Config:
    orm_mode = True  # replaced by model_config = ConfigDict(from_attributes=True)

@validator("field")  # replaced by @field_validator
```

Pydantic v2 has a compatibility layer but it emits deprecation warnings. Use v2 native patterns: `ConfigDict`, `field_validator`, `model_validator`.

## info: interactive docs enabled in production

`/docs` and `/redoc` expose the full API surface to unauthenticated users in production. Disable with `docs_url=None, redoc_url=None, openapi_url=None` unless you need them and they are access-controlled.

## info: no response_model_exclude_unset on PATCH endpoints

Returning all fields on a PATCH response, including fields the caller did not set, forces clients to merge state client-side. Use `response_model_exclude_unset=True` to return only the fields that were updated.
