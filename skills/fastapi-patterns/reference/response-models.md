# Response models

- A route's response: declare `response_model=UserRead` on the decorator. FastAPI validates the return value and serializes only `UserRead`'s fields, so an ORM object's `hashed_password` never reaches the response. The model also sets the response's OpenAPI schema.
- A PATCH route that returns only the fields it changed: `response_model_exclude_unset=True` on the decorator, and apply the update from `update.model_dump(exclude_unset=True)`.
- When write and read shapes differ, take `UserCreate` as the body and return through `response_model=UserRead`. The schema boundary drops `password`, not filtering logic in the handler.
- `response_model` and a `-> UserRead` return annotation: use both. Both feed the OpenAPI docs; `response_model` applies FastAPI's filtering and validation, and type checkers read the annotation.
- A non-200 success code: `status_code=` on the decorator, as a `fastapi.status` constant (`status.HTTP_201_CREATED`, `status.HTTP_204_NO_CONTENT`), not a bare integer.
