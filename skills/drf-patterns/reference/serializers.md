# Serializers

## Explicit fields

Always declare an explicit `fields` list. `fields = "__all__"` is a warning: it exposes every column including ones added later that should stay internal.

```python
class ProductSerializer(serializers.ModelSerializer):
    class Meta:
        model = Product
        fields = ["id", "name", "price", "slug"]  # explicit
```

DRF has required either `fields` or `exclude` since version 3.5 (deprecated in 3.3.0). Omitting both raises an error at startup.

## Validation

Field-level: `validate_<field_name>(self, value)` - runs after the field's own validation, receives a single value.

Object-level: `validate(self, data)` - runs after all field validators succeed, receives the full data dict. Use for cross-field constraints.

```python
class OrderSerializer(serializers.ModelSerializer):
    class Meta:
        model = Order
        fields = ["quantity", "start_date", "end_date"]

    def validate_quantity(self, value):
        if value <= 0:
            raise serializers.ValidationError("Quantity must be positive.")
        return value

    def validate(self, data):
        if data["start_date"] >= data["end_date"]:
            raise serializers.ValidationError("start_date must be before end_date.")
        return data
```

Never do authorization checks in `validate()`. That belongs in `permission_classes`.

## Nested serializers

Declare another serializer as a field for nested representation:

```python
class OrderSerializer(serializers.ModelSerializer):
    product = ProductSerializer(read_only=True)
    product_id = serializers.PrimaryKeyRelatedField(
        queryset=Product.objects.all(), source="product", write_only=True
    )
    class Meta:
        model = Order
        fields = ["id", "product", "product_id", "quantity"]
```

Writable nested serializers require explicit `.create()` and `.update()` implementations. DRF does not auto-handle nested writes.

## SerializerMethodField

`SerializerMethodField` calls `get_<field_name>(self, obj)` for computed read-only fields:

```python
class ProductSerializer(serializers.ModelSerializer):
    avg_rating = serializers.SerializerMethodField()

    def get_avg_rating(self, obj):
        return obj.reviews.aggregate(avg=Avg("rating"))["avg"]
```

Avoid putting database queries inside `SerializerMethodField` on list endpoints without prefetching.

## Separate input and output schemas

When the write shape differs significantly from the read shape, use separate serializers. One serializer that handles both often ends up with awkward `read_only` / `write_only` juggling that obscures intent.

```python
class ProductCreateSerializer(serializers.ModelSerializer):
    class Meta:
        model = Product
        fields = ["name", "price", "category_id"]

class ProductReadSerializer(serializers.ModelSerializer):
    category = CategorySerializer(read_only=True)
    class Meta:
        model = Product
        fields = ["id", "name", "price", "category", "slug"]
```

## References

- https://www.django-rest-framework.org/api-guide/serializers/
- https://www.django-rest-framework.org/api-guide/serializers/#modelserializer
- https://www.django-rest-framework.org/api-guide/fields/#serializermethodfield
