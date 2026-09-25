# Common patterns

## Contents

- [Value objects and enums](#value-objects-and-enums)
- [Pathlib, context managers, caching](#pathlib-context-managers-caching)
- [Pydantic v2 at trust boundaries](#pydantic-v2-at-trust-boundaries)

## Value objects and enums

- `@dataclass(frozen=True, slots=True)` is the default for value objects (`slots=True` is 3.10+). Mutable dataclasses only when mutation is the point.
- `enum.Enum` for closed sets. `enum.StrEnum` (3.11+) when the enum value must serialize as its own string (JSON, query params, log fields).

## Pathlib, context managers, caching

- `pathlib.Path` over `os.path`. `Path("dir") / "file.txt"`, `path.read_text()`, `path.glob("*.py")` -- all clearer than the string-juggling equivalents.
- Context managers: `@contextlib.contextmanager` for one-off setup/teardown; full `__enter__` / `__exit__` class for reusable infrastructure.
- `functools.cache` (3.9+) for unbounded memoization.
- `functools.lru_cache(maxsize=N)` when the cache must be bounded.
- `itertools.pairwise(it)` (3.10+) for adjacent pairs.
- `itertools.batched(it, n)` (3.12+) for fixed-size chunks.
- Both `itertools` helpers replace common hand-rolled loops.

## Pydantic v2 at trust boundaries

When data crosses a trust boundary (HTTP body, file, queue message, config file), validate it with a schema before letting it propagate. Pydantic v2 is the standard choice; FastAPI builds on it and the broader ecosystem followed.

Method names changed between v1 and v2; mixing them is a common source of confusion. The v2 names:

- `Model.model_validate(data)` parses untrusted input into a validated model. Replaces v1 `parse_obj`.
- `Model.model_validate_json(data)` parses a JSON string directly. Replaces v1 `parse_raw`.
- `instance.model_dump()` serializes a model instance to a `dict`. Replaces v1 `dict()`.
- `instance.model_dump_json()` serializes to a JSON string. Replaces v1 `json()`.

Idiomatic field declaration uses `Annotated[]` for compositional metadata:

```python
from typing import Annotated
from pydantic import BaseModel, Field

class CreateUser(BaseModel):
    email: Annotated[str, Field(min_length=3, max_length=254)]
    age: Annotated[int, Field(ge=0, le=150)]
```

Discriminated unions use `Field(discriminator="kind")` (or whichever field carries the literal):

```python
from typing import Literal
from pydantic import BaseModel, Field

class Cat(BaseModel):
    kind: Literal["cat"]
    purrs_at: int

class Dog(BaseModel):
    kind: Literal["dog"]
    barks_at: int

class Pet(BaseModel):
    pet: Annotated[Cat | Dog, Field(discriminator="kind")]
```

The discriminator gives Pydantic an O(1) dispatch on parse and a clear error message when the discriminator value is unknown. Without it, Pydantic walks the union and reports the first member that fails to validate, which is usually not what the caller wants to see.

Construct branded / validated values only via the schema. The mistake to avoid is sharing a model class between "input shape" (untrusted) and "internal value" (trusted); use two models and a parse step between them.

## References

- Python `What's New` (current stable): https://docs.python.org/3/whatsnew/
- Pydantic v2 models: https://pydantic.dev/docs/validation/latest/concepts/models/
- Pydantic v2 fields: https://pydantic.dev/docs/validation/latest/concepts/fields/
- Pydantic v2 unions: https://pydantic.dev/docs/validation/latest/concepts/unions/
