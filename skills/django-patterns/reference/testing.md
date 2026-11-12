# Testing

## pytest-django

- A pytest codebase: pytest-django (4.14.0), in the style `skills/test-patterns/reference/pytest.md` sets.
- A test that touches the database: `@pytest.mark.django_db`. Without it, the test can't.
- Pure logic (a validator, a service function): no `django_db` mark. Keep those tests fast and unconditional.
- A large migration history: `pytest --reuse-db`, which reuses the test database and skips `migrate`. In CI, once migrations are settled, `--no-migrations`.

## Fixture tools

- Factory Boy (3.3.3) or model_bakery: use one, not both. Both are actively maintained.
- Complex object graphs with interdependencies: Factory Boy (`factory.django.DjangoModelFactory`, `factory.SubFactory`, `factory.Faker`).
- Quick, simple objects with auto-generated values: model_bakery, as in `baker.make("myapp.Order", total=100)`.

## What to test

- Django's own behavior, such as `filter(x=1)` returning rows where `x=1`, is a framework default: `skills/test-patterns/reference/coverage-and-skip.md`.
