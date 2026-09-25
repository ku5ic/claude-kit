---
name: logging-patterns
description: >
  Python logging patterns covering stdlib logging configuration, structured
  logging with structlog, log levels, handlers, formatters, and context binding.
  Use whenever a project uses Python logging, structlog, or the user asks about
  log levels, log formatting, structured logs, or JSON logging, even if
  "logging" is not mentioned by name.
---

# Logging Patterns

Review checklist plus the configuration shapes that are easy to get wrong. Level semantics and handler constructors are in the stdlib docs.

## stdlib gotchas

- `basicConfig` is a no-op if the root logger already has handlers. Call it once at the entry point; pass `force=True` to replace existing handlers (useful in tests).
- Records propagate up the dot-separated hierarchy to parent handlers. Set `propagate = False` only when you have deliberately attached a handler to that logger - otherwise you get every record twice.
- Silence a noisy dependency by name rather than lowering the global level: `logging.getLogger("httpx").setLevel(logging.WARNING)`.

## structlog configuration

The processor chain is order-sensitive and the API shifts between minor versions - pin this shape and verify against the docs on upgrade.

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

- Request-scoped context that survives awaits and task boundaries goes through `structlog.contextvars.bind_contextvars(...)`, which `merge_contextvars` folds into every record. `logger.bind(...)` returns a new logger and does not cross task boundaries.
- Use the `a`-prefixed calls (`ainfo`, `aerror`) in async code so rendering does not block the loop.
- `structlog.stdlib.recreate_defaults()` routes structlog through stdlib logging - needed when third-party libraries log too.

---

## Anti-patterns

**failure: f-strings in log calls**
`logger.warning(f"msg: {a}")` interpolates even when the record is filtered out. Use `logger.warning("msg: %s", a)` for stdlib, or `logger.warning("msg", key=a)` for structlog.

**failure: bare except with logging.exception inside a loop**
Catching all exceptions and logging them without re-raising hides bugs in long-running loops. Either re-raise or catch a specific exception type.

**warning: configuring logging inside library code**
Libraries must not call `basicConfig`, `addHandler`, or `setLevel` at module level. Configure only in the application entry point. Libraries add a `NullHandler` to their top-level logger and nothing else.

**warning: print() for diagnostic output in server code**
`print` bypasses handlers, formatters, and level filters. Replace with a logger at the appropriate level.

**warning: root logger used directly in library modules**
`logging.info(...)` logs to the root logger and cannot be scoped or silenced per-module. Always use `logging.getLogger(__name__)`.

**info: no structured context on error logs**
Plain string messages like `"payment failed"` are hard to aggregate. Add identifiers: `logger.error("payment.failed", order_id=order.id, amount=amount)`.

---

## References

- https://docs.python.org/3/library/logging.html
- https://docs.python.org/3/howto/logging.html
- https://www.structlog.org/en/stable/getting-started.html

> Verify structlog processor names and configuration shape against the structlog docs when upgrading, as the processor API evolves between minor versions.
