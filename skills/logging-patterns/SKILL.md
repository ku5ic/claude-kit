---
name: logging-patterns
description: Python logging patterns for stdlib config, structlog, levels, handlers, and JSON logs. Load before any edit to Python logging or structlog code, however small, and for logging questions.
---

# Logging Patterns

Review checklist plus the configuration shapes that are easy to get wrong. Level semantics and handler constructors are in the stdlib docs.

## stdlib gotchas

- `basicConfig`: call it once, at the entry point. It's a no-op once the root logger has handlers; pass `force=True` to replace them, as in tests.
- Set `propagate = False` only on a logger you deliberately attached a handler to.
- A noisy dependency: raise its own logger's level by name, `logging.getLogger("httpx").setLevel(logging.WARNING)`, never the global level.

## structlog configuration

- Keep the processors in this order. On a structlog upgrade, check their names and the config shape against the docs.

```python
import logging
import structlog

structlog.configure(
    processors=[
        structlog.contextvars.merge_contextvars,
        structlog.processors.add_log_level,
        structlog.processors.StackInfoRenderer(),
        structlog.dev.set_exc_info,
        structlog.processors.TimeStamper(fmt="iso", utc=True),
        structlog.dev.ConsoleRenderer(),  # JSONRenderer() in production
    ],
    wrapper_class=structlog.make_filtering_bound_logger(logging.DEBUG),
    logger_factory=structlog.PrintLoggerFactory(),
    cache_logger_on_first_use=True,
)
```

- Request-scoped context that must survive awaits and task boundaries: bind it with `structlog.contextvars.bind_contextvars(...)`, which `merge_contextvars` folds into every record. `logger.bind(...)` returns a new logger and doesn't cross task boundaries.
- Async code: use the `a`-prefixed calls (`ainfo`, `aerror`).
- When third-party libraries log too, route structlog through stdlib logging with `structlog.stdlib.recreate_defaults()`.

## Anti-patterns

- An f-string in a log call, `logger.warning(f"msg: {a}")`: `failure`. Use `logger.warning("msg: %s", a)` in stdlib, `logger.warning("msg", key=a)` in structlog.
- A long-running loop that catches all exceptions, logs them with `logger.exception`, and doesn't re-raise: `failure`. Re-raise, or catch a specific exception type.
- Library code that calls `basicConfig`, `addHandler`, or `setLevel` at module level: `warning`. Configure logging only at the application entry point; a library adds a `NullHandler` to its top-level logger and nothing else.
- `print()` for diagnostic output in server code: `warning`. Use a logger at the right level.
- A library module that logs through the root logger, `logging.info(...)`: `warning`. Use `logging.getLogger(__name__)`.
- An error log with no identifiers, such as `"payment failed"`: `info`. Add them: `logger.error("payment.failed", order_id=order.id, amount=amount)`.
