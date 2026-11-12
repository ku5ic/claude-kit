# pytest

- Mirror the source layout under `tests/`, one test module per source module.
- Put fixtures in `conftest.py` at the appropriate scope. Use function scope unless setup is expensive; `module` and `session` scopes share state and couple test order.
- Use `monkeypatch` for test-local changes to env vars, attributes, and module-level singletons; it reverts them after the test.
- Write table tests with `@pytest.mark.parametrize`, and give each case an `id`.
- Django: `skills/django-patterns/reference/testing.md`.
- In a pytest codebase, don't subclass `unittest.TestCase`. Pick one style.
