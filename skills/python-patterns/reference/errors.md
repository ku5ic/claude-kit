# Errors

- Exceptions as control flow on a hot path: since 3.11 the `try` success path is effectively zero-cost (bpo-40222), but a raise still allocates frames. Use LBYL when the check is cheap, EAFP (`try`/`except`) when it isn't.
- Catch specific exception types. `except Exception:` only at the top of a request or job boundary.
- Custom errors: a base `AppError` with named subclasses (`ValidationError`, `NotFoundError`). Callers catch a category, never match a message string.
- Raising a new error from a caught one: `raise NewError(...) from original`.
- Cleanup: `with` for a context-managed resource, `try/finally` otherwise.
- Logging inside `except`: `logger.exception("message")`, never `logger.error(str(e))`.
