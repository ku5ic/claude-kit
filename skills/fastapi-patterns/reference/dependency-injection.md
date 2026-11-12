# Dependency injection

- Declare a route parameter that comes from a dependency function, not the body or path, as `Annotated[T, Depends(fn)]`, never `param: T = Depends(fn)`.
- A dependency that needs cleanup (database session, file handle, external connection): `yield` the resource inside `try`/`finally`. The code after `yield` runs after the response is sent.
- An exception from the route reaches the yield dependency's `finally`. Don't catch and swallow it there unless you mean to change the error behavior.
- A dependency that needs configuration can be a class. `Annotated[PaginationParams, Depends()]`, with no argument, injects the class itself and calls its `__init__` with query params.
- A dependency whose side effects must repeat within one request: `Depends(fn, use_cache=False)`. By default FastAPI runs each dependency once per request and shares the result.
- A dependency swapped in tests: `app.dependency_overrides[get_db] = override_get_db`, with no change to route code. Reset it with `app.dependency_overrides = {}` after the test or test module that set it.
