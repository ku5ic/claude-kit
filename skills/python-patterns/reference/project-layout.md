# Project layout, packaging, and imports

## Contents

- [Project layout](#project-layout)
- [Packaging and imports](#packaging-and-imports)
- [The py.typed marker](#the-pytyped-marker)
- [Namespace packages](#namespace-packages)

## Project layout

- `src/` layout for libraries and any project that will be packaged. PyPA documents three benefits, the load-bearing one being that it prevents accidentally importing the in-development source instead of the installed package. Flat layout is fine for scripts and small applications.
- `pyproject.toml` is the single source of truth. PEP 518 standardizes `[build-system]`; PEP 621 standardizes `[project]` metadata. No `setup.py` for new projects.
- Tool config consolidated in `pyproject.toml`: `[tool.ruff]`, `[tool.mypy]`, `[tool.pyright]`, `[tool.pytest.ini_options]`. Avoid scattering across `setup.cfg`, `mypy.ini`, `pytest.ini` unless a tool requires it.
- Dependency manager:
  - uv: faster, single-tool replacement for pip/pip-tools/pipx/poetry/pyenv/twine/virtualenv.
  - Poetry: established, 2.x reads dependencies from `[project]` per PEP 621.
  - Both ship lockfiles (`uv.lock`, `poetry.lock`) -- commit them.
  - `requirements.txt` is acceptable as a deployment artifact, not as a development workflow.

## Packaging and imports

- Absolute imports inside packages. Relative imports (`from .util import x`) acceptable only for tight intra-package coupling.
- `__init__.py` empty unless re-exporting is intentional. A package that re-exports from submodules is making an API promise; document it.
- `__all__` only when publishing a public API. It controls `from package import *` and signals intent to type-checkers and IDEs.
- Circular imports: detect with Ruff (rules in the `TID`/`F` series), `pylint`, or `pydeps` for visualizing the import graph. Fix by moving the shared symbol to a third module, not by deferring imports inside functions.

## The py.typed marker

A package that ships type annotations must include an empty `py.typed` file at the package root for type checkers to honor those annotations. Without it, type checkers treat the package as untyped even when the source has annotations everywhere, and downstream consumers see implicit `Any` for every imported symbol. The mechanism is PEP 561.

The file applies recursively to all subpackages of the package that contains it. Stub-only distributions use `<package>-stubs` directory naming and the same marker. Third-party libraries that exist without `py.typed` (still common) need an external stub package to type-check cleanly; absence of `py.typed` is itself the signal that types are not part of the public API.

For first-party packages: ship `py.typed` from day one, and configure the build (setuptools, hatch, poetry, uv) to include it in the distribution. Forgetting to include the marker in the wheel is a common publishing miss.

## Namespace packages

A directory without `__init__.py` is treated as an implicit namespace package (PEP 420). Multiple directories on the path can contribute to the same package name; the import system aggregates them into one logical package without any of them owning an `__init__.py`.

Use case: monorepo plugins where each plugin lives in its own directory but imports under one parent name (`mycompany.plugins.payments`, `mycompany.plugins.shipping`, etc.). Each plugin's directory contributes to the `mycompany.plugins` namespace; no central `__init__.py` exists.

Use carefully. Mixing namespace packages with regular packages of the same name across the path silently breaks tooling that walks `__init__.py` (Sphinx autodoc, some packaging tools). When in doubt, prefer regular packages; reach for namespace packages only when the multi-distribution split is the actual requirement.

## References

- PyPA src vs flat layout: https://packaging.python.org/en/latest/discussions/src-layout-vs-flat-layout/
- PEP 518 (build system requirements): https://peps.python.org/pep-0518/
- PEP 621 (project metadata): https://peps.python.org/pep-0621/
- PEP 561 (typed package marker): https://peps.python.org/pep-0561/
- PEP 420 (implicit namespace packages): https://peps.python.org/pep-0420/
- uv docs: https://docs.astral.sh/uv/
- Poetry docs: https://python-poetry.org/docs/
