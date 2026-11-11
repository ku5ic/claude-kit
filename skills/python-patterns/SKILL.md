---
name: python-patterns
description: Python patterns for typing, project layout, async, errors, and packaging. Use for .py files, pyproject.toml, requirements.txt, uv.lock, poetry.lock, or Python questions.
---

# Python patterns

Default assumption: Python 3.11 or later, type hints required on public function signatures, mypy or pyright in strict mode.

- 3.11 is the floor: `TaskGroup`, `Self`, `StrEnum`, and `ExceptionGroup` all landed there.
- 3.15 is current stable (since 2026-10-09). 3.14 made deferred annotation evaluation the default; on older supported versions, `from __future__ import annotations` is still useful for forward references.
- Adapt advice to the version in the project's `.tool-versions`, `pyproject.toml`, or `.python-version`.

## Reference files

| File                                                         | Covers                                                                                         |
| ------------------------------------------------------------ | ---------------------------------------------------------------------------------------------- |
| [reference/typing.md](reference/typing.md)                   | Type hints, strictness, mypy/pyright per-module overrides                                      |
| [reference/project-layout.md](reference/project-layout.md)   | `src/` layout, `pyproject.toml`, packaging, `py.typed` (PEP 561), namespace packages (PEP 420) |
| [reference/async.md](reference/async.md)                     | `async def` hazards, `TaskGroup`, `anyio` cancellation scopes                                  |
| [reference/errors.md](reference/errors.md)                   | Exception design, chaining, LBYL vs EAFP, `logger.exception`                                   |
| [reference/common-patterns.md](reference/common-patterns.md) | Dataclasses, enums, `pathlib`, `functools.cache`, Pydantic v2 patterns                         |
| [reference/anti-patterns.md](reference/anti-patterns.md)     | Thirteen language anti-patterns, free-threaded build implications, testing-adjacent items      |
