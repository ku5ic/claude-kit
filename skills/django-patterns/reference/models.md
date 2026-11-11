# Models

## Fields

- `null=True` is database-level (stores NULL) and `blank=True` is validation-level (allows empty in forms). They're independent.
- A char field: `blank=True` without `null=True`, so "no value" has one representation.
- `ForeignKey` and `OneToOneField` require `on_delete`. Choose it deliberately:
  - `CASCADE` deletes the child with the parent.
  - `SET_NULL` sets the FK to NULL and needs `null=True`.
  - `PROTECT` blocks deleting a parent that still has children.
  - `DO_NOTHING` leaves the FK dangling, and the DB constraint is yours to handle.
- `ForeignKey` and `ManyToManyField`: always set an explicit `related_name`, never rely on the default `<model>_set`.
- Give every field used in admin or forms a `help_text`.

## Constraints and indexes

- A cross-field rule: a `Meta.constraints` entry, not model `clean()` alone. For example `models.UniqueConstraint(fields=["person", "group"], name="unique_person_group")` or `models.CheckConstraint(condition=models.Q(quantity__gte=0), name="non_negative_quantity")`.
- A field in `filter()`, `order_by()`, or `exclude()` on a large table gets a `Meta.indexes` entry, such as `models.Index(fields=["status", "created_at"])`. Profile before adding one; every index costs on write.

## Managers

- A custom manager subclasses `models.Manager`. Override `get_queryset()` for a default filter, or add named methods for named queries.
- A filtering manager such as `active = ActiveManager()` sits beside `objects = models.Manager()`, the default.
- Table-level queries go in manager methods and row-level computation in model methods. Never mix them.
- Business logic: model methods and service functions, never views. Views handle request and response shape only.
