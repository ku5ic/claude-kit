# Async

## Contents

- [Core async hazards](#core-async-hazards)
- [Structured concurrency with TaskGroup](#structured-concurrency-with-taskgroup)
- [anyio for cross-runtime libraries](#anyio-for-cross-runtime-libraries)

## Core async hazards

- `async def` is contagious. Sync I/O in an async function blocks the event loop. `time.sleep` in async is `failure`; use `await asyncio.sleep`. Synchronous `requests` in async is `warning`; use `httpx` (sync + async API) or `aiohttp`.
- `asyncio.gather(*aws)` for parallel I/O without structured cancellation. `asyncio.TaskGroup` (3.11+, motivated by PEP 654) for structured concurrency: cancellations propagate, and child failures are collected as an `ExceptionGroup`.
- `asyncio.to_thread(func, *args)` (3.9+) wraps a sync blocking call so it runs in a thread without blocking the loop.
- `async with` for async context managers, `async for` for async iterators. Do not call sync `open(...)` on a hot path inside async; use `aiofiles` or hand off via `to_thread`.
- Never call `asyncio.run(...)` inside a running event loop. From a coroutine, `await` directly; from sync code already inside a loop, schedule via `loop.create_task` or `asyncio.ensure_future`.
- `anyio` (v4) when the code must run under both asyncio and trio. Useful for libraries; over-engineering for application code that owns its loop.

## Structured concurrency with TaskGroup

`TaskGroup` is an async context manager that owns its child tasks. Three guarantees the standard library docs make explicit:

1. All tasks added via `tg.create_task(...)` are awaited at `__aexit__`.
2. The first child failure (other than `CancelledError`) cancels every other task in the group.
3. Multiple failed tasks are surfaced as an `ExceptionGroup` raised by the `async with` block; one failure does not silently mask the others.

```python
async def fetch_all(urls: list[str]) -> list[bytes]:
    results: list[bytes] = []
    async with asyncio.TaskGroup() as tg:
        tasks = [tg.create_task(client.get(u)) for u in urls]
    return [t.result() for t in tasks]
```

Compare to `asyncio.gather(*tasks, return_exceptions=False)`: gather will cancel siblings but does not collect multiple errors into an `ExceptionGroup`, and a leaked task created inside the gather call is not tied to its lexical scope. Reach for `gather` only when one of those properties is the goal.

Cancellation semantics: a `CancelledError` raised inside the `async with` body is treated as the cooperative shutdown signal. The group cancels its children, awaits their cleanup, then re-raises. Code inside the body should not catch `CancelledError` indiscriminately; doing so swallows the shutdown signal and leaks tasks.

## anyio for cross-runtime libraries

`anyio` provides asyncio-and-trio-compatible primitives, the most useful being cancel scopes.

```python
async with anyio.move_on_after(5.0):
    await long_running_call()
# returns normally on timeout (no exception); use fail_after to raise instead
```

Two semantic differences from raw `asyncio` worth knowing:

- **Level cancellation.** A task in a cancelled scope receives `CancelledError` at every yield point until it exits the scope, not just once. This makes "I'll catch the cancel and keep working" patterns impossible by construction.
- **Nested scopes.** Scopes nest cleanly; cancelling an outer scope cancels every inner scope. This is the structured-concurrency primitive the language `TaskGroup` formalizes for asyncio specifically.

For application code that owns its event loop (web servers, CLI tools), `asyncio.TaskGroup` is enough. Reach for `anyio` when shipping a library that callers may run under either asyncio or trio; the cancel-scope semantics also benefit code with deep timeout discipline.

## References

- `asyncio.TaskGroup`: https://docs.python.org/3/library/asyncio-task.html#asyncio.TaskGroup
- PEP 654 (exception groups): https://peps.python.org/pep-0654/
- httpx: https://www.python-httpx.org/
- aiohttp: https://docs.aiohttp.org/en/stable/
- anyio cancellation: https://anyio.readthedocs.io/en/stable/cancellation.html
