# Async

## asyncio

- Concurrent awaits on 3.11+: `asyncio.TaskGroup` (motivated by PEP 654). Use `asyncio.gather` only when its behavior is the goal: siblings keep running after one raises, errors aren't collected into an `ExceptionGroup`, and its tasks aren't tied to a lexical scope.
- What a `TaskGroup` guarantees: every `tg.create_task(...)` is awaited at `__aexit__`, the first child failure other than `CancelledError` cancels the rest, and several failures surface together as an `ExceptionGroup` from the `async with`.
- A sync blocking call inside `async def` on 3.9+: `await asyncio.to_thread(func, *args)`.
- Async context managers and iterators: `async with`, `async for`.
- Never call `asyncio.run(...)` inside a running loop. From a coroutine, `await` directly; from sync code already inside a loop, `loop.create_task` or `asyncio.ensure_future`.
- Code inside a `TaskGroup` body: never catch `CancelledError` indiscriminately.

## anyio

- anyio (v4): only when code must run under both asyncio and trio, such as a library, or for code with deep timeout discipline. Application code that owns its loop uses `asyncio.TaskGroup`.
- `with anyio.move_on_after(5.0):` is a plain `with` that returns normally on timeout. To raise instead, use `fail_after`.
- anyio cancellation is level-triggered: a task in a cancelled scope gets `CancelledError` at every yield point until it leaves the scope.
- anyio scopes nest; cancelling an outer scope cancels every inner one.
