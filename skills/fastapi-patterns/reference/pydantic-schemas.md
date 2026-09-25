# Pydantic Schemas

## Contents

- [BaseModel](#basemodel)
- [Field()](#field)
- [field_validator](#field_validator)
- [model_validator](#model_validator)
- [model_config and ConfigDict](#model_config-and-configdict)
- [Separate input and output schemas](#separate-input-and-output-schemas)
- [References](#references)

Pydantic v2.13.x. Core module is Rust-based; performance is not a reason to avoid validation.

## BaseModel

Subclass `BaseModel` for all request and response schemas. Type hints drive both validation and serialization.

```python
from pydantic import BaseModel

class ProductCreate(BaseModel):
    name: str
    price: float
    sku: str | None = None
```

## Field()

Use `Field()` to add constraints and metadata beyond type hints:

```python
from pydantic import BaseModel, Field

class ProductCreate(BaseModel):
    name: str = Field(min_length=1, max_length=200, description="Display name")
    price: float = Field(gt=0, description="Price in USD")
    quantity: int = Field(ge=0, default=0)
    sku: str | None = Field(default=None, pattern=r"^[A-Z]{2}-\d{4}$")
    tags: list[str] = Field(default_factory=list)
```

Supported constraints: `gt`, `lt`, `ge`, `le` (numeric), `min_length`, `max_length`, `pattern` (string), `default`, `default_factory`, `title`, `description`.

## field_validator

`field_validator` runs on a single field. Default mode is `after` (post-type-coercion). Use `before` to preprocess raw input.

```python
from pydantic import BaseModel, field_validator

class OrderCreate(BaseModel):
    quantity: int
    email: str

    @field_validator("quantity", mode="after")
    @classmethod
    def quantity_positive(cls, value: int) -> int:
        if value <= 0:
            raise ValueError("quantity must be positive")
        return value

    @field_validator("email", mode="before")
    @classmethod
    def normalize_email(cls, value: object) -> object:
        if isinstance(value, str):
            return value.lower().strip()
        return value
```

`mode='before'` receives raw input (any type); `mode='after'` receives the already-validated typed value.

## model_validator

`model_validator` runs on the full object. Use for cross-field constraints.

```python
from typing_extensions import Self
from pydantic import BaseModel, model_validator

class DateRange(BaseModel):
    start: date
    end: date

    @model_validator(mode="after")
    def check_order(self) -> Self:
        if self.start >= self.end:
            raise ValueError("start must be before end")
        return self
```

`mode='after'` receives the validated model instance. `mode='before'` receives raw data as `dict | Any` and is a `@classmethod`.

## model_config and ConfigDict

Configure model behavior via `model_config`:

```python
from pydantic import BaseModel, ConfigDict

class ProductRead(BaseModel):
    model_config = ConfigDict(
        from_attributes=True,   # enables ORM mode (replaces orm_mode in v1)
        populate_by_name=True,  # accept both alias and field name
        validate_default=True,  # validate default values
        str_strip_whitespace=True,
    )
    id: int
    name: str
```

`from_attributes=True` is required to build a schema from a SQLAlchemy model instance.

## Separate input and output schemas

When write and read shapes differ, use separate schemas. One schema with mixed `read_only`/`write_only` juggling obscures intent and allows mistakes.

```python
class UserCreate(BaseModel):
    username: str
    email: str
    password: str

class UserRead(BaseModel):
    model_config = ConfigDict(from_attributes=True)
    id: int
    username: str
    email: str
    # password intentionally absent
```

Use `UserCreate` as the route body type and `UserRead` as the `response_model`. This makes the password-exclusion contract explicit at the type level, not a runtime filter.

## References

- https://pydantic.dev/docs/validation/latest/
- https://pydantic.dev/docs/validation/latest/concepts/validators/
- https://pydantic.dev/docs/validation/latest/concepts/fields/
