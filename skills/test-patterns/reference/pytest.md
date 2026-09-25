# pytest (Django and general Python)

- File layout: mirror source layout under `tests/`. One test module per source module.
- Fixtures in `conftest.py` at the appropriate scope. Prefer function scope unless setup is expensive.
- Parametrize with `@pytest.mark.parametrize` for table tests. Include an `id` for readability on failure.
- Django: use `pytest-django`, `@pytest.mark.django_db` only when DB needed. Prefer `--no-migrations` in CI if migrations are settled.
- Factory Boy for model fixtures when the same model is used in many tests.
- Avoid `unittest.TestCase` subclasses in a pytest codebase. Pick one style.

## References

- pytest: https://docs.pytest.org/en/stable/
- pytest-django: https://pytest-django.readthedocs.io/
- Factory Boy: https://factoryboy.readthedocs.io/en/stable/
