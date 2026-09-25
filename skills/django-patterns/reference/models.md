# Models

## Field options

`null=True` is database-level (stores NULL). `blank=True` is validation-level (allows empty in forms). They are orthogonal. A char field should typically use `blank=True` without `null=True` to avoid two representations of "no value".

`on_delete` is required on `ForeignKey` and `OneToOneField`. Choose deliberately:

- `CASCADE` - delete child when parent is deleted
- `SET_NULL` - set to NULL (requires `null=True`)
- `PROTECT` - prevent deletion of parent if children exist
- `DO_NOTHING` - leave the FK dangling (requires manual DB constraint handling)

See the full list at docs.djangoproject.com/en/stable/ref/models/fields/#django.db.models.ForeignKey.on_delete.

Always set an explicit `related_name` on `ForeignKey` and `ManyToManyField`. The default `<model>_set` breaks on model rename and is ambiguous when multiple FKs point at the same model.

## Constraints and indexes

Use `Meta.constraints` for cross-field rules. Database constraints are stronger than model `clean()` because they catch bulk inserts and `update()` calls that bypass model validation.

```python
class Meta:
    constraints = [
        models.UniqueConstraint(
            fields=["person", "group"],
            name="unique_person_group"
        ),
        models.CheckConstraint(
            condition=models.Q(quantity__gte=0),
            name="non_negative_quantity"
        ),
    ]
```

Use `Meta.indexes` on fields that appear in `filter()`, `order_by()`, or `exclude()` on large tables. Profile before adding; indexes cost on write.

```python
class Meta:
    indexes = [
        models.Index(fields=["status", "created_at"]),
    ]
```

## Managers

The default manager is `objects`. Custom managers extend `models.Manager` and override `get_queryset()` for default filtering, or add named methods for named query patterns.

Manager methods for table-level queries. Model methods for row-level computation. Do not mix.

```python
class ActiveManager(models.Manager):
    def get_queryset(self):
        return super().get_queryset().filter(is_active=True)

class Product(models.Model):
    objects = models.Manager()
    active = ActiveManager()
```

## Structural patterns

Fat models over fat views: business logic belongs in model methods and service functions, not in views. Views handle request/response shape only.

`help_text` on fields used in admin or forms. It doubles as inline documentation.

## References

- https://docs.djangoproject.com/en/stable/topics/db/models/
- https://docs.djangoproject.com/en/stable/ref/models/fields/
- https://docs.djangoproject.com/en/stable/ref/models/options/
