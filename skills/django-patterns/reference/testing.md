# Testing

## pytest-django

Use pytest-django (v4.14.0) over Django's `TestCase` in a pytest codebase. Tests are plain functions, not subclasses. Less boilerplate.

```python
import pytest
from myapp.models import Product

@pytest.mark.django_db
def test_product_slug_is_generated():
    product = Product.objects.create(name="Widget")
    assert product.slug == "widget"
```

`@pytest.mark.django_db` requests database access for that test. Tests without it cannot touch the database.

## Fixture tools

Factory Boy (v3.3.3) and model_bakery are both actively maintained. Use one, not both.

Factory Boy is better for complex object graphs with interdependencies:

```python
import factory
from myapp.models import Order

class OrderFactory(factory.django.DjangoModelFactory):
    class Meta:
        model = Order
    user = factory.SubFactory(UserFactory)
    total = factory.Faker("pydecimal", left_digits=4, right_digits=2, positive=True)
```

model_bakery is better for quick, simple object creation with auto-generated values:

```python
from model_bakery import baker

def test_order_can_be_fulfilled():
    order = baker.make("myapp.Order", total=100)
    assert order.can_be_fulfilled()
```

Stop writing `Model.objects.create(field=..., field2=..., ...)` in every test. Use a factory.

## What to test

`@pytest.mark.django_db` only when needed. Unit tests on pure logic (a validator, a service function) do not need the database. Keep those tests fast and unconditional.

Do not test Django itself. Do not write tests that verify `filter(x=1)` returns rows where `x=1`. Test your application's logic, not the ORM.

## Speed

`--reuse-db` (pytest-django flag) reuses the test database between runs, skipping the `migrate` step. Significant time savings on large migration histories.

```bash
pytest --reuse-db
```

## References

- https://pytest-django.readthedocs.io/en/latest/
- https://factoryboy.readthedocs.io/en/stable/
- https://model-bakery.readthedocs.io/en/latest/
