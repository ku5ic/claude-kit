# Anti-patterns

- Pydantic v1 patterns in a v2 project (`class Config: orm_mode = True`, `@validator`): `warning`. Use `model_config = ConfigDict(from_attributes=True)`, `@field_validator`, and `@model_validator`.
- A blocking call, such as a sync ORM query, in an `async def` route: `failure`. Make the route `def`, or switch to an async ORM.
- `return HTTPException(...)`: `failure`. Raise it.
- A route returning an ORM object with no `response_model`: `failure`. Declare `response_model=UserRead`.
- `detail=str(exc)` on a database or third-party exception: `failure`. Log the exception and return a generic message.
- A mutating route (POST, PUT, PATCH, DELETE) with no auth dependency: `warning`, unless the route is explicitly public. Depend on the auth dependency.
- An API key read with `APIKeyQuery`: `warning`. Use `APIKeyHeader`.
- `app.dependency_overrides` set in a test and never reset: `warning`. Reset it after the test or test module that sets it.
- `/docs` and `/redoc` on in production: `info`. Set `docs_url`, `redoc_url`, and `openapi_url` to `None` unless they're needed and access-controlled.
- A PATCH response with every field, including ones the caller didn't set: `info`. Use `response_model_exclude_unset=True`.
