# Migrations

## Writing migrations

- Names: descriptive, `makemigrations --name add_product_sku_index` for `0042_add_product_sku_index`. An auto name like `0042_auto_20260101_0101` is only acceptable for an initial migration.
- PostgreSQL and SQLite run each migration in one transaction by default. MySQL and Oracle don't support DDL transactions, so theirs run without one.
- A migration that can't run in a transaction, such as a concurrent index on Postgres: `atomic = False` on the `Migration` class.
- Data migrations: `RunPython`, with models from `apps.get_model("auth", "User")`, never direct imports.
- `RunPython`: pass a reverse function as the second argument, or `migrations.RunPython.noop` when the data migration isn't reversible.
- An applied migration: never edit it. Add a new one.

## CI and squashing

- CI: `python manage.py makemigrations --check`, which exits non-zero on model changes that have no migration.
- A history large enough to slow test setup significantly: `squashmigrations`. It optimizes operations (`CreateModel` and `DeleteModel` cancel out) and writes a replacement migration with a `replaces` attribute.
- After squashing, keep the old migrations until every deployment has run the squashed one, then remove them in a follow-up.
