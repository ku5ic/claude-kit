# ORM

## select_related vs prefetch_related

`select_related` uses a SQL JOIN. Use it for forward `ForeignKey` and `OneToOneField` lookups.

```python
# 1 query instead of N+1
entries = Entry.objects.select_related("blog").all()
for e in entries:
    print(e.blog.name)  # no extra query
```

`prefetch_related` runs a separate batch query per relationship and joins in Python. Use it for reverse foreign keys, `ManyToManyField`, and `GenericRelation`.

```python
# 2 queries: one for Pizza, one for toppings
for pizza in Pizza.objects.prefetch_related("toppings"):
    print(pizza.toppings.all())  # no extra query
```

Filtering after a prefetch bypasses the cache and runs a new query. Use `Prefetch(queryset=...)` to filter at prefetch time instead.

N+1 detection: django-debug-toolbar's SQL panel shows duplicate queries in development. Measure before optimizing.

## F() expressions

`F()` references a field value at the database level. Use it for atomic increments and cross-field comparisons.

```python
from django.db.models import F

# Atomic: no race condition
Reporter.objects.filter(name="Tintin").update(stories_filed=F("stories_filed") + 1)

# Cross-field filter
Company.objects.filter(num_employees__gt=F("num_chairs"))
```

Never do `obj.field += 1; obj.save()` under concurrent writes. Two processes reading the same value will both increment from the same base.

## Q() objects

`Q()` encapsulates a filter condition and supports `|` (OR), `&` (AND), `~` (NOT), `^` (XOR).

```python
from django.db.models import Q

Poll.objects.filter(
    Q(question__startswith="Who") | Q(question__startswith="What")
)
```

Q objects must come before keyword arguments in `filter()` and `get()` calls. Reversing the order raises a `TypeError`.

## Column-level optimization

`.only(*fields)` fetches only the named columns. `.defer(*fields)` fetches all columns except the named ones. Both return model instances with deferred fields loaded on access.

Use when fetching many rows but reading few columns. Measure with django-debug-toolbar before adding.

## Large querysets

`.iterator()` streams rows from the database instead of loading the full result set into memory. Useful for queryset sizes that would otherwise OOM.

```python
for obj in LargeModel.objects.iterator(chunk_size=1000):
    process(obj)
```

Note: `iterator()` does not use the queryset cache. Iterating a second time hits the database again.

## Bulk operations

`QuerySet.bulk_create([...])` inserts multiple rows in fewer queries. `QuerySet.bulk_update([...], fields)` updates multiple rows without fetching each first.

Both bypass `Model.save()` and post-save signals. Know what you are skipping.

## References

- https://docs.djangoproject.com/en/stable/ref/models/querysets/#select-related
- https://docs.djangoproject.com/en/stable/ref/models/querysets/#prefetch-related
- https://docs.djangoproject.com/en/stable/ref/models/expressions/#f-expressions
- https://docs.djangoproject.com/en/stable/topics/db/queries/#complex-lookups-with-q-objects
- https://docs.djangoproject.com/en/stable/ref/models/querysets/#iterator
- https://docs.djangoproject.com/en/stable/ref/models/querysets/#bulk-create
