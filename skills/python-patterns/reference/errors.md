# Errors

- Specific exception types. Bare `except:` is `failure` (it catches `KeyboardInterrupt` and `SystemExit`). `except Exception:` only at the top of a request / job boundary.
- Custom hierarchy: a base `AppError` with named subclasses (`ValidationError`, `NotFoundError`, etc.). Lets callers handle categories without string-matching messages.
- `raise NewError(...) from original` to chain. Preserves the cause; tracebacks show both.
- `try/finally` for cleanup; `with` for context-managed resources. A `with` block is almost always cleaner than the matching `try/finally`.
- Exceptions for control flow on hot paths: 3.11 made the success path effectively zero-cost (bpo-40222), but raised exceptions still allocate frames. `LBYL` (look-before-you-leap) when the check is cheap; `EAFP` (try/except) when it isn't.
- `logger.exception("message")` inside an `except` block. It includes the traceback; `logger.error(str(e))` does not.
