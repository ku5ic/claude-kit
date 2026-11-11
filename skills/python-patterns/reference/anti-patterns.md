# Anti-patterns to flag

## Free-threaded Python and module-level state

- The free-threaded build: experimental in 3.13 (`python3.13t`, PEP 703), officially supported in 3.14 (PEP 779) with a roughly 5-10% single-threaded penalty depending on platform. Neither release makes it the default, and installing C-extension packages on it needs pip 24.1+.
- Code that runs on the free-threaded build: put a `threading.Lock` on module-level mutable state the GIL used to serialize, or use `threading.local` when the state is per-thread.
- State that needs the lock: module-level dicts and lists mutated by worker threads (`_cache`, `_registry`, `_handlers`), lazy-init sentinels (`if _value is None: _value = compute()`), and class-level mutable defaults shared across instances.
- Porting a library to the free-threaded build: the no-GIL build's behavior is the spec.

## Language-level anti-patterns

- Module-level mutable state (`_cache = {}`) without synchronization: `warning`, doubly so under free-threaded 3.13+.
- `from x import *` outside test fixtures or the REPL: `warning`.
- `== None` or `!= None`: `warning`. Use `is None` or `is not None`.
- `if value:` where empty and missing differ: `warning`. It's false for `0`, `""`, `[]`, `{}`, and `None`, so write `if value is None` or `if not value` on purpose.
- A sync blocking call (DB query, slow file I/O, third-party SDK) inside `async def`: `warning`. Use `await asyncio.to_thread(func, ...)` or the library's async API.
- `dict.get(key)` then a `None` check where `KeyError` is the right signal: `info`.
- An `Optional[X] = None` parameter callers must always provide: `info`. Use `X`.
- A class with only `__init__` and one method: `info`. Make it a function.
- An inheritance hierarchy 3+ levels tall for code reuse: `info`. Use composition.

## Testing-adjacent

pytest conventions (fixtures, scope, parametrize, factories) are test-patterns' `reference/pytest.md`; Django test setup is django-patterns' `reference/testing.md`.
