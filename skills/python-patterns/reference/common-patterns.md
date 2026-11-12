# Common patterns

## Standard library

- Fixed-size chunks on 3.12+: `itertools.batched(it, n)`, not a hand-rolled loop.
- An enum value that must serialize as its own string (JSON, query params, log fields) on 3.11+: `enum.StrEnum`. Other closed sets: `enum.Enum`.
- Adjacent pairs on 3.10+: `itertools.pairwise(it)`, not a hand-rolled loop.
- Value objects: `@dataclass(frozen=True, slots=True)` (`slots` is 3.10+). A mutable dataclass only when mutation is the point.
- Unbounded memoization on 3.9+: `functools.cache`. Bounded: `functools.lru_cache(maxsize=N)`.
- Paths: `pathlib.Path` (`Path("dir") / "file.txt"`, `path.read_text()`, `path.glob("*.py")`), not `os.path`.
- Context managers: `@contextlib.contextmanager` for one-off setup and teardown, a full `__enter__`/`__exit__` class for reusable infrastructure.

## Pydantic v2 at trust boundaries

- Data crossing a trust boundary (HTTP body, file, queue message, config file): validate it with a Pydantic v2 schema before it propagates.
- v2 method names, never the v1 ones: `Model.model_validate(data)` (was `parse_obj`), `Model.model_validate_json(data)` (`parse_raw`), `instance.model_dump()` (`dict()`), `instance.model_dump_json()` (`json()`).
- Field constraints: `Annotated[]` metadata, as in `age: Annotated[int, Field(ge=0, le=150)]`.
- A union of models: a discriminated union, `pet: Annotated[Cat | Dog, Field(discriminator="kind")]`, where each member declares `kind: Literal["cat"]` and so on.
- Validated values: construct them only through the schema. Untrusted input and trusted internal values get two models with a parse step between them, never one shared class.
