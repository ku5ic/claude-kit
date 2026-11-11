# python-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- PEP index: https://peps.python.org/
- Python `What's New` (current stable): https://docs.python.org/3/whatsnew/
- mypy command line: https://mypy.readthedocs.io/en/stable/command_line.html
- mypy config (per-module overrides): https://mypy.readthedocs.io/en/stable/config_file.html
- pyright configuration: https://microsoft.github.io/pyright/
- Ruff rules: https://docs.astral.sh/ruff/rules/
- uv docs: https://docs.astral.sh/uv/
- PyPA src vs flat: https://packaging.python.org/en/latest/discussions/src-layout-vs-flat-layout/
- Pydantic v2 models: https://pydantic.dev/docs/validation/latest/concepts/models/

### Version notes

Checked: 2026-11-10 against https://www.python.org/downloads/, https://docs.python.org/3.15/whatsnew/3.15.html and https://docs.python.org/3/library/asyncio-task.html

- 3.15 (2026-10-09, latest 3.15.2): typing additions PEP 747 `TypeForm`, PEP 728 `TypedDict(closed=True, extra_items=...)`, PEP 800 disjoint bases; PEP 810 lazy imports; UTF-8 default encoding (PEP 686). Not yet guidance here. Also corrected: `gather` does not cancel siblings, `anyio.move_on_after` is a plain `with`, zero-cost `try` is bpo-40222.
- 3.14 (2025-10-07, latest 3.14.10): deferred annotation evaluation by default (PEP 649); `from __future__ import annotations` on a deprecation path (PEP 749); free-threaded build officially supported (PEP 779), 5-10% single-threaded penalty, not the default build.
- 3.13: free-threaded build experimental (PEP 703). Once 3.13 reaches EOL the floor here moves and the `from __future__` paragraph can go.

## reference/anti-patterns.md

### References

- Ruff rules: https://docs.astral.sh/ruff/rules/
- Python 3.13 free-threaded build: https://docs.python.org/3/whatsnew/3.13.html#free-threaded-cpython
- Python 3.14 free-threaded improvements: https://docs.python.org/3/whatsnew/3.14.html
- PEP 703 (optional GIL): https://peps.python.org/pep-0703/
- PEP 779 (official support): https://peps.python.org/pep-0779/

## reference/async.md

### References

- `asyncio.TaskGroup`: https://docs.python.org/3/library/asyncio-task.html#asyncio.TaskGroup
- PEP 654 (exception groups): https://peps.python.org/pep-0654/
- httpx: https://www.python-httpx.org/
- aiohttp: https://docs.aiohttp.org/en/stable/
- anyio cancellation: https://anyio.readthedocs.io/en/stable/cancellation.html

## reference/common-patterns.md

### References

- Python `What's New` (current stable): https://docs.python.org/3/whatsnew/
- Pydantic v2 models: https://pydantic.dev/docs/validation/latest/concepts/models/
- Pydantic v2 fields: https://pydantic.dev/docs/validation/latest/concepts/fields/
- Pydantic v2 unions: https://pydantic.dev/docs/validation/latest/concepts/unions/

## reference/errors.md

### References

- PEP 654 (exception groups): https://peps.python.org/pep-0654/
- PEP 657 (fine-grained tracebacks): https://peps.python.org/pep-0657/
- Python tutorial (errors and exceptions): https://docs.python.org/3/tutorial/errors.html

## reference/project-layout.md

### References

- PyPA src vs flat layout: https://packaging.python.org/en/latest/discussions/src-layout-vs-flat-layout/
- PEP 518 (build system requirements): https://peps.python.org/pep-0518/
- PEP 621 (project metadata): https://peps.python.org/pep-0621/
- PEP 561 (typed package marker): https://peps.python.org/pep-0561/
- PEP 420 (implicit namespace packages): https://peps.python.org/pep-0420/
- uv docs: https://docs.astral.sh/uv/
- Poetry docs: https://python-poetry.org/docs/

## reference/typing.md

### References

- PEP 585 (built-in generics): https://peps.python.org/pep-0585/
- PEP 649 (deferred annotation evaluation): https://peps.python.org/pep-0649/
- PEP 673 (`Self`): https://peps.python.org/pep-0673/
- PEP 695 (type parameter syntax): https://peps.python.org/pep-0695/
- mypy command line: https://mypy.readthedocs.io/en/stable/command_line.html
- mypy config (per-module overrides): https://mypy.readthedocs.io/en/stable/config_file.html
- pyright configuration: https://microsoft.github.io/pyright/
- Ruff rules: https://docs.astral.sh/ruff/rules/
