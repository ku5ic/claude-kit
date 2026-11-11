# Project layout, packaging, and imports

## Project layout

- Poetry 2.x: dependencies go in `[project]` (PEP 621).
- A library, or any project that gets packaged: `src/` layout. Flat layout is fine for scripts and small applications.
- Project metadata and build config: `pyproject.toml` only, `[build-system]` (PEP 518) and `[project]` (PEP 621). No `setup.py` in a new project.
- Tool config: in `pyproject.toml` (`[tool.ruff]`, `[tool.mypy]`, `[tool.pyright]`, `[tool.pytest.ini_options]`), not `setup.cfg`, `mypy.ini`, or `pytest.ini`, unless a tool requires it.
- Dependency manager: uv, which replaces pip, pip-tools, pipx, poetry, pyenv, twine, and virtualenv, or Poetry.
- Lockfiles (`uv.lock`, `poetry.lock`): commit them.
- `requirements.txt`: a deployment artifact only, never the development workflow.

## Packaging and imports

- Imports inside a package are absolute. Use relative ones (`from .util import x`) only for tight intra-package coupling.
- `__init__.py`: empty unless re-exporting on purpose. A re-export is an API promise; document it.
- `__all__`: only when publishing a public API.
- A circular import: move the shared symbol to a third module, never defer the import inside a function. Find them with Ruff (`TID`/`F` rules), `pylint`, or `pydeps`.

## The py.typed marker

- A package that ships annotations: an empty `py.typed` at the package root (PEP 561) from day one, and a build config (setuptools, hatch, poetry, uv) that puts it in the wheel.
- `py.typed` covers every subpackage. A stub-only distribution: a `<package>-stubs` directory with the same marker.
- A third-party library without `py.typed`: type-check it through an external stub package.

## Namespace packages

- A directory without `__init__.py` is an implicit namespace package (PEP 420): several directories on the path can contribute to one package name.
- Namespace packages: only when a multi-distribution split is the requirement, such as monorepo plugins under one parent (`mycompany.plugins.payments`). Otherwise, regular packages.
- Never mix a namespace package and a regular package of the same name across the path.
