# Pydantic schemas

- Pydantic v2's core is Rust: performance is no reason to skip validation.
- A schema built from a SQLAlchemy model instance: `model_config = ConfigDict(from_attributes=True)`, which replaces v1's `orm_mode`.
- Request and response schemas subclass `BaseModel`, and their type hints drive both validation and serialization.
- Constraints and metadata beyond the type: `Field()`. Numeric `gt`, `lt`, `ge`, `le`; string `min_length`, `max_length`, `pattern`; plus `default`, `default_factory`, `title`, `description`.
- A check on one field: `@field_validator("name")` stacked on `@classmethod`. The default `mode="after"` gets the coerced, typed value; `mode="before"` gets raw input of any type, for preprocessing.
- A cross-field check: `@model_validator(mode="after")`, which gets the validated instance and returns `Self`. `mode="before"` gets raw data as `dict | Any` and is a `@classmethod`.
- Other model behavior: `model_config = ConfigDict(...)`, with options such as `populate_by_name=True` (accept alias and field name), `validate_default=True` (validate default values), and `str_strip_whitespace=True`.
- Write and read shapes differ: separate schemas, `UserCreate` as the route body and `UserRead`, with no `password` field, as the `response_model`. Never one schema juggling `read_only`/`write_only` fields.
