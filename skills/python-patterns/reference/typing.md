# Type hints and strictness

## Type hints

- Forward references and self-referential types on 3.11-3.13: `from __future__ import annotations`. On 3.14+, drop it: deferred evaluation is the default (PEP 649) and the import is on a deprecation path (PEP 749).
- Generics with a 3.12+ floor: PEP 695 syntax, `type Pair[T] = tuple[T, T]`, `class Container[T]:`, `class Repo[T: BaseClass]:`. Supporting 3.11: `TypeVar("T", bound=BaseClass)` plus `Generic[T]`.
- A fluent API or factory method on 3.11+: return `typing.Self` (PEP 673), not a `TypeVar` bound to the class.
- Internal value objects: `@dataclass(frozen=True, slots=True)` (`slots` is 3.10+).
- Dict shapes at trust boundaries (config files, JSON payloads): `TypedDict`.
- Validation that's part of the contract: `pydantic.BaseModel` (v2).
- Structural typing: `Protocol`, not `abc.ABC`.
- Runtime-introspectable hints: `Annotated[T, metadata]`, as in `PositiveInt = Annotated[int, Gt(0)]`.

## Strictness

- Baseline: mypy `--strict` or pyright `strict`. Strict off on a package is a `failure` for any new code path.
- mypy `--strict` enables, among others, `--disallow-untyped-defs`, `--disallow-any-generics`, `--warn-return-any`, `--warn-unused-ignores`, `--strict-equality`, `--no-implicit-reexport`.
- A suppression: `# type: ignore[error-code]`, never a bare `# type: ignore`.
- `Any` in a public signature: `failure`. Internal `Any`: only where interop demands it, with a one-line comment.
- `cast(X, value)`: only with the runtime invariant documented next to the call.
- Linting and formatting a new project: Ruff, one tool with two subcommands, in place of Flake8 and its plugins, Black, isort, pydocstyle, pyupgrade, and autoflake.

## Type-checker configuration

- Strictness that can't be uniform: first-party code stays under `--strict`; legacy modules and unstubbed libraries get per-module overrides.
- mypy overrides: `[[tool.mypy.overrides]]` tables in `pyproject.toml`, each with a `module` pattern (string or list) and any subset of mypy options, such as `disallow_untyped_defs = false` or `ignore_missing_imports = true`.
- Precedence: inline `# type: ignore[code]` beats config, a concrete module name beats a wildcard, a wildcard beats globals.
- Raising strictness: turn the flag on globally, add a temporary override for the modules that fail, then remove the overrides one at a time as those modules get typed.
- pyright: `pyrightconfig.json` or a `[tool.pyright]` table, with per-path `executionEnvironments`, same workflow.
