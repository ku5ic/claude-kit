# Migrations

## Naming

Name migrations descriptively: `0042_add_product_sku_index`, not `0042_auto_20260101_0101`. The auto-generated name is only acceptable for initial migrations.

```bash
python manage.py makemigrations --name add_product_sku_index
```

## Atomic behavior

PostgreSQL and SQLite run migrations inside a single transaction by default. MySQL and Oracle do not support DDL transactions, so their migrations run without one.

To disable transactions for a migration that requires it (e.g., creating a concurrent index in Postgres):

```python
class Migration(migrations.Migration):
    atomic = False
```

## Data migrations with RunPython

Use `RunPython` for data migrations. Access models through the historical app registry (`apps.get_model()`), not direct imports. Direct imports fail when the migration is replayed on a schema that does not match the current model definition.

```python
def backfill_display_name(apps, schema_editor):
    User = apps.get_model("auth", "User")
    for user in User.objects.all():
        user.display_name = f"{user.first_name} {user.last_name}"
        user.save()

class Migration(migrations.Migration):
    operations = [
        migrations.RunPython(backfill_display_name, migrations.RunPython.noop),
    ]
```

Provide a reverse function (or `migrations.RunPython.noop` if the data migration is not reversible) as the second argument.

## CI gate

`makemigrations --check` exits non-zero when there are model changes without a corresponding migration. Use it in CI to prevent drift:

```bash
python manage.py makemigrations --check
```

## Squashing

Squash when the migration history has grown large enough to slow down test setup significantly. The `squashmigrations` command optimizes operations (e.g., `CreateModel` + `DeleteModel` cancel each other out) and writes a replacement migration with a `replaces` attribute.

Keep the old migrations alongside the squashed one until all deployments have run it. Then remove the old files in a follow-up.

Never edit an applied migration. Add a new one.

## References

- https://docs.djangoproject.com/en/stable/topics/migrations/
- https://docs.djangoproject.com/en/stable/ref/django-admin/#makemigrations
- https://docs.djangoproject.com/en/stable/howto/writing-migrations/
