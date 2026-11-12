# Error handling

- An error response: `raise HTTPException(status_code=..., detail=...)`. Never `return` it.
- A status code in `HTTPException`: a `fastapi.status` constant (`status.HTTP_404_NOT_FOUND`), not a bare integer.
- Extra headers on an error response, such as `WWW-Authenticate`: pass `headers={...}` to `HTTPException`.
- An application exception many routes raise: register `@app.exception_handler(MyError)` returning a `JSONResponse`, and let routes raise it without catching it.
- A handler for every `HTTPException`: register one for `starlette.exceptions.HTTPException` as well, or override the default handler. A handler for FastAPI's `HTTPException` alone misses the Starlette ones that middleware and routing raise.
- Never put stack traces, database error messages, internal paths or file names, or raw third-party exception messages in `detail`. Log the error with `logger.exception(...)` and raise with a generic message.
- Pydantic validation failures: FastAPI returns 422 with per-field locations and messages, no code needed. To change that body's shape, override the `RequestValidationError` handler from `fastapi.exceptions`.
