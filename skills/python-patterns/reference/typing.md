# Type hints and strictness

## Contents

- [Type hints](#type-hints)
- [Strictness](#strictness)
- [Type-checker configuration patterns](#type-checker-configuration-patterns)

## Type hints

- Built-in generics over `typing` aliases. `list[str]`, `dict[str, int]`, `tuple[int, ...]`. PEP 585, 3.9+.
- `from typing import Self` for fluent APIs and factory methods (3.11+, PEP 673). Cleaner than `TypeVar` bounded on the class.
- `from __future__ import annotations` on 3.11-3.13 to turn annotations into strings -- helps with forward references and self-referential types. On 3.14 this is unnecessary and on a deprecation path (PEP 749); deferred evaluation is the default (PEP 649).
- New generic syntax in 3.12+ (PEP 695): `type Pair[T] = tuple[T, T]` and `class Container[T]:` instead of `TypeVar("T")` + `Generic[T]`. Use it when the floor is 3.12; keep `TypeVar` when supporting 3.11.
- `TypedDict` for dict shapes at trust boundaries (config files, JSON payloads).
- `dataclass(frozen=True, slots=True)` for internal value objects (`slots=True` is 3.10+).
- `pydantic.BaseModel` (v2) when validation is part of the contract.
- `Protocol` over `abc.ABC` for structural typing. Lets callers satisfy the type without inheriting.
- `Annotated[T, metadata]` for runtime-introspectable hints. Pydantic v2 and FastAPI use this heavily (e.g. `PositiveInt = Annotated[int, Gt(0)]`).
- Generic constraints: `T = TypeVar("T", bound=BaseClass)` for the legacy syntax; `class Repo[T: BaseClass]:` for the PEP 695 form.

## Strictness

- mypy `--strict` or pyright `strict` mode is the baseline. Strict-off on a package is a `failure` for any new code path.
- mypy `--strict` is an umbrella; among other flags it enables `--disallow-untyped-defs`, `--disallow-any-generics`, `--warn-return-any`, `--warn-unused-ignores`, `--strict-equality`, `--no-implicit-reexport`.
- `# type: ignore[error-code]` over bare `# type: ignore`. The bracketed form documents the intent and lets `--warn-unused-ignores` catch stale suppressions.
- `Any` in a public function signature is a `failure`. Internal `Any` with a one-line comment is acceptable when interop demands it.
- `cast(X, value)` is a smell unless the runtime invariant is documented adjacent to the call.
- Ruff is the linter and formatter for new Python projects. It consolidates Flake8 (plus plugins), Black, isort, pydocstyle, pyupgrade, autoflake, with 800+ rules. Linter and formatter are one tool, two subcommands.

## Type-checker configuration patterns

Strictness is rarely uniform across a codebase. Per-module overrides let you keep first-party code under `--strict` while granting a softer regime to legacy modules or third-party libraries that ship without stubs.

mypy in `pyproject.toml`:

```toml
[tool.mypy]
strict = true

[[tool.mypy.overrides]]
module = "legacy_package.*"
disallow_untyped_defs = false
warn_return_any = false

[[tool.mypy.overrides]]
module = ["unstubbed_lib", "another_lib.submodule"]
ignore_missing_imports = true
```

The `[[tool.mypy.overrides]]` table accepts a `module` pattern (string or list) and any subset of mypy options. Inline `# type: ignore[code]` overrides any config; concrete module names override wildcards; wildcards override globals. This precedence order lets you ratchet strictness incrementally: turn a flag on globally, then add a temporary override for the modules that fail, and remove the overrides one at a time as the modules are typed.

pyright uses a `pyrightconfig.json` (or a `[tool.pyright]` table) with per-path `executionEnvironments` for the same effect. Both checkers support a similar tightening workflow; the keys differ but the strategy does not.

## References

- PEP 585 (built-in generics): https://peps.python.org/pep-0585/
- PEP 649 (deferred annotation evaluation): https://peps.python.org/pep-0649/
- PEP 673 (`Self`): https://peps.python.org/pep-0673/
- PEP 695 (type parameter syntax): https://peps.python.org/pep-0695/
- mypy command line: https://mypy.readthedocs.io/en/stable/command_line.html
- mypy config (per-module overrides): https://mypy.readthedocs.io/en/stable/config_file.html
- pyright configuration: https://microsoft.github.io/pyright/
- Ruff rules: https://docs.astral.sh/ruff/rules/
