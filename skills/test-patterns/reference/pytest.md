# pytest (Django and general Python)

- Mirror the source layout under `tests/`, one test module per source module.
- Put fixtures in `conftest.py` at the appropriate scope. Use function scope unless setup is expensive; `module` and `session` scopes share state and couple test order.
- Use `monkeypatch` for test-local changes to env vars, attributes, and module-level singletons; it reverts them after the test.
- Write table tests with `@pytest.mark.parametrize`, and give each case an `id`.
- Django: use `pytest-django`, and mark `@pytest.mark.django_db` only on tests that need the DB. Prefer `--no-migrations` in CI once migrations are settled.
- Build model fixtures with Factory Boy when the same model is used in many tests.
- In a pytest codebase, don't subclass `unittest.TestCase`. Pick one style.
