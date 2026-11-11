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

Read every file in this table as soon as this skill loads. A hook blocks every tool call until you have.

| Before                                                                             | Read                                                         |
| ---------------------------------------------------------------------------------- | ------------------------------------------------------------ |
| Adding type hints, or setting mypy or pyright strictness                           | [reference/typing.md](reference/typing.md)                   |
| Laying out a package, editing `pyproject.toml`, or organizing imports              | [reference/project-layout.md](reference/project-layout.md)   |
| Writing async code, or running blocking work from it                               | [reference/async.md](reference/async.md)                     |
| Raising, catching, or chaining an exception                                        | [reference/errors.md](reference/errors.md)                   |
| Using `itertools`, an enum, a dataclass, `functools.cache`, `pathlib`, or Pydantic | [reference/common-patterns.md](reference/common-patterns.md) |
| Reviewing Python code                                                              | [reference/anti-patterns.md](reference/anti-patterns.md)     |
