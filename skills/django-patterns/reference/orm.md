# ORM

## select_related and prefetch_related

- Forward `ForeignKey` and `OneToOneField` lookups: `select_related`, which uses one SQL JOIN.
- Reverse foreign keys, `ManyToManyField`, and `GenericRelation`: `prefetch_related`, which runs one batch query per relation and joins in Python.
- A filtered prefetch: `Prefetch(queryset=...)` at prefetch time. Filtering after the prefetch skips the cache and runs a new query.
- N+1 queries: find them with django-debug-toolbar's SQL panel in development, and measure before optimizing.

## F() and Q()

- Atomic increments and cross-field comparisons: `F()`, as in `.update(stories_filed=F("stories_filed") + 1)` or `.filter(num_employees__gt=F("num_chairs"))`.
- Under concurrent writes, never `obj.field += 1; obj.save()`. Use an `F()` update.
- Combined filter conditions: `Q()` objects with `|` (OR), `&` (AND), `~` (NOT), and `^` (XOR).

## Columns and large querysets

- Many rows with few columns read: `.only(*fields)` or `.defer(*fields)`. Deferred fields load on access. Measure with django-debug-toolbar before adding either.
- A result set too large for memory: `.iterator(chunk_size=1000)`, which streams rows. It skips the queryset cache, so a second loop queries again.

## Bulk operations

- Many inserts: `bulk_create([...])`. Many updates without fetching each row: `bulk_update([...], fields)`.
- Both skip `Model.save()` and post-save signals. Know what you're skipping.
