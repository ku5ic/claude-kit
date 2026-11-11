# Serializers

## Explicit fields

- Always declare an explicit `fields` list in `Meta`. Since 3.5, omitting both `fields` and `exclude` raises an error at startup.
- `fields = "__all__"` is a `warning`.

## Validation

- Field-level checks: `validate_<field_name>(self, value)`, which runs after the field's own validation and receives one value.
- Cross-field constraints: `validate(self, data)`, which runs after every field validator succeeds and receives the full data dict.
- Raise `serializers.ValidationError` on failure; return the value or the data otherwise.
- Never check authorization in `validate()`. That belongs in `permission_classes`.

## Nested serializers

- Nested representation: declare another serializer as a field, as in `product = ProductSerializer(read_only=True)`. To write the relation, add a `PrimaryKeyRelatedField` with `source="product"` and `write_only=True`.
- A writable nested serializer needs explicit `.create()` and `.update()` implementations. DRF doesn't handle nested writes.

## SerializerMethodField

- A computed read-only field: `SerializerMethodField`, which calls `get_<field_name>(self, obj)`.
- On a list endpoint, never put a database query inside a `SerializerMethodField` without prefetching.

## Separate input and output schemas

- When the write shape differs significantly from the read shape, use separate serializers, such as `ProductCreateSerializer` and `ProductReadSerializer`, not one serializer juggling `read_only` and `write_only`.
