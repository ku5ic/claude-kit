---
name: python-patterns
description: Python language-level patterns - type hints, strictness, project layout, async, error handling, packaging, and review-worthy anti-patterns. Use whenever the project contains `.py` files or a Python manifest (`pyproject.toml`, `requirements.txt`, `Pipfile`, `uv.lock`, `poetry.lock`), OR the user asks about Python, its typing and type checkers, its async model, or its packaging and project structure, even if "Python" is not mentioned by name.
---

# Python patterns

Default assumption: Python 3.11 or later, type hints required on public function signatures, mypy or pyright in strict mode.

- 3.11 is the floor: `TaskGroup`, `Self`, `StrEnum`, and `ExceptionGroup` all landed there.
- 3.14 (current stable since 2025-10-07, latest 3.14.7) made deferred annotation evaluation the default; on older supported versions, `from __future__ import annotations` is still useful for forward references.
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

## References

- PEP index: https://peps.python.org/
- Python `What's New` (current stable): https://docs.python.org/3/whatsnew/
- mypy command line: https://mypy.readthedocs.io/en/stable/command_line.html
- mypy config (per-module overrides): https://mypy.readthedocs.io/en/stable/config_file.html
- pyright configuration: https://microsoft.github.io/pyright/
- Ruff rules: https://docs.astral.sh/ruff/rules/
- uv docs: https://docs.astral.sh/uv/
- PyPA src vs flat: https://packaging.python.org/en/latest/discussions/src-layout-vs-flat-layout/
- Pydantic v2 models: https://pydantic.dev/docs/validation/latest/concepts/models/

## Version notes

Checked: 2026-09-12 against https://www.python.org/downloads/ and https://docs.python.org/3/whatsnew/

- 3.15 (pre-release, planned 2026-10-01): typing additions PEP 747 `TypeForm`, PEP 728 `TypedDict(closed=True, extra_items=...)`, PEP 800 disjoint bases. Not yet guidance here.
- 3.14 (2025-10-07, latest 3.14.7 on 2026-08-05): deferred annotation evaluation by default (PEP 649); `from __future__ import annotations` on a deprecation path (PEP 749); free-threaded build officially supported (PEP 779), 5-10% single-threaded penalty, not the default build.
- 3.13: free-threaded build experimental (PEP 703). Once 3.13 reaches EOL the floor here moves and the `from __future__` paragraph can go.
