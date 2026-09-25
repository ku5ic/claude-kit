# Anti-patterns to flag

## Contents

- [Language-level anti-patterns](#language-level-anti-patterns)
- [Free-threaded Python and module-level state](#free-threaded-python-and-module-level-state)
- [Testing-adjacent](#testing-adjacent)

## Language-level anti-patterns

- Mutable default arguments: `def f(x=[]):`, `def f(x={}):`. The default is shared across calls. `failure`. Use `None` and reassign inside the body.
- Bare `except:` or `except Exception: pass`. Errors disappear silently. `failure`.
- `from x import *` outside test fixtures or REPL. Pollutes the namespace; defeats lints. `warning`.
- `== None`, `!= None`. Use `is None` / `is not None`. `warning`. (Ruff `E711`.)
- Truthiness check where empty-vs-missing matters. `if value:` is False for `0`, `""`, `[]`, `{}`, AND `None`. Be explicit: `if value is None`, `if not value`, etc. `warning`.
- `dict.get(key)` followed by a `None` check when `KeyError` would be the correct signal. The exception is louder than a missed branch. `info`.
- `Optional[X] = None` parameter that callers must always provide. The type is lying. `info`. Use `X` and let mypy enforce.
- `time.sleep(...)` in `async def`. Blocks the event loop. `failure`. Use `await asyncio.sleep(...)`.
- Synchronous `requests.get/post` in `async def`. Blocks the event loop. `warning`. Use `httpx.AsyncClient` or `aiohttp.ClientSession`.
- Calling a sync blocking function (DB query, slow file I/O, third-party SDK) directly inside `async def`. `warning`. Wrap with `await asyncio.to_thread(func, ...)` or use the library's async API.
- Class with only `__init__` and one method. Should be a function. `info`.
- Module-level mutable state (`_cache = {}`) without synchronization. Concurrency hazard, doubly so under free-threaded 3.13+. `warning`.
- Tall inheritance hierarchies (3+ levels of `class A(B(C)):`) for code reuse. Prefer composition. `info`.

## Free-threaded Python and module-level state

PEP 703 introduced an experimental no-GIL build in Python 3.13 (`python3.13t`); 3.14 graduated it to officially supported per PEP 779, with a roughly 5-10% single-threaded performance penalty depending on platform. The runtime is not the default in either release; opt-in builds and pip 24.1+ for installing C-extension packages remain the gating story.

The implication for module-level mutable state: code that "happened to be safe" because the GIL serialized every bytecode is no longer safe under the free-threaded interpreter. Common patterns now needing a lock:

- Module-level dict / list mutated by multiple worker threads (`_cache`, `_registry`, `_handlers`).
- Lazy-initialization sentinels (`if _value is None: _value = compute()`).
- Class-level mutable defaults shared across instances.

Add a `threading.Lock` (or use `threading.local` if the state is genuinely per-thread) before reading code into the free-threaded build. The CPython documentation for 3.13/3.14 spells this out; treat the no-GIL build's behavior as the definitive spec when porting libraries to the new interpreter.

## Testing-adjacent

For test code review; depth lives in test-runner-specific references.

- `pytest` over `unittest.TestCase`. Fewer ceremonies; better fixtures; better parametrize.
- Fixture scope: `function` (default) unless setup is genuinely expensive. `module` / `session` scopes share state and create order coupling.
- `monkeypatch` for clean test-local mutations of env vars, attributes, and module-level singletons. Auto-reverts.
- `@pytest.mark.parametrize("input,expected", [...], ids=[...])`. The `ids` argument keeps test names readable when the inputs are objects.
- Factory Boy (`factory.django.DjangoModelFactory`) or model_bakery for Django model fixtures. Stop hand-writing `Model.objects.create(...)` in every test.

## References

- Ruff rules: https://docs.astral.sh/ruff/rules/
- Python 3.13 free-threaded build: https://docs.python.org/3/whatsnew/3.13.html#free-threaded-cpython
- Python 3.14 free-threaded improvements: https://docs.python.org/3/whatsnew/3.14.html
- PEP 703 (optional GIL): https://peps.python.org/pep-0703/
- PEP 779 (official support): https://peps.python.org/pep-0779/
