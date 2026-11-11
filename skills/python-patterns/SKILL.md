---
name: python-patterns
description: Python patterns for typing, project layout, async, errors, and packaging. Use for .py files, pyproject.toml, requirements.txt, uv.lock, poetry.lock, or Python questions.
---

# Python patterns

Default assumption: Python 3.11 or later, type hints required on public function signatures, mypy or pyright in strict mode.

- 3.15 is current stable (since 2026-10-09).
- 3.14 made deferred annotation evaluation the default. On older supported versions, use `from __future__ import annotations` for forward references.
- 3.11 is the floor: `TaskGroup`, `Self`, `StrEnum`, and `ExceptionGroup` all landed there.
- Adapt advice to the version in the project's `.tool-versions`, `pyproject.toml`, or `.python-version`.

## Reference files

| File                                                         | Covers                                                                               |
| ------------------------------------------------------------ | ------------------------------------------------------------------------------------ |
| [reference/typing.md](reference/typing.md)                   | Version-gated hints (`__future__`, PEP 695, `Self`), strictness, mypy/pyright config |
| [reference/project-layout.md](reference/project-layout.md)   | `src/` layout, `pyproject.toml`, imports, `py.typed` (PEP 561), namespace packages   |
| [reference/async.md](reference/async.md)                     | `TaskGroup` vs `gather`, `to_thread`, `asyncio.run`, `anyio` cancel scopes           |
| [reference/errors.md](reference/errors.md)                   | LBYL vs EAFP, exception types, chaining, cleanup, `logger.exception`                 |
| [reference/common-patterns.md](reference/common-patterns.md) | `itertools`, enums, dataclasses, `functools.cache`, `pathlib`, Pydantic v2           |
| [reference/anti-patterns.md](reference/anti-patterns.md)     | Free-threaded build, nine language anti-patterns, testing-adjacent pointers          |
