# Views and Forms

## Class-based vs function-based views

Both are valid. The choice is a project-level convention, not a quality signal.

CBVs (class-based views) are good for CRUD flows with standard shapes: `ListView`, `DetailView`, `CreateView`, `UpdateView`, `DeleteView`. Less code, but the inheritance chain is deep and harder to trace.

FBVs (function-based views) are good for one-off endpoints, complex logic, and when readability matters more than code reduction. Easier to understand, test, and debug.

Pick one per area and stay consistent. Mixing both styles for equivalent endpoints creates confusion.

## Thin views

Views handle request parsing and response shaping only. Business logic belongs in services or model methods.

```python
def create_order(request):
    form = OrderForm(request.POST)
    if form.is_valid():
        order = order_service.create(form.cleaned_data, user=request.user)
        return redirect(order.get_absolute_url())
    return render(request, "orders/create.html", {"form": form})
```

## get_object_or_404

Use `get_object_or_404` for single object lookups. Do not catch `DoesNotExist` manually.

```python
from django.shortcuts import get_object_or_404

product = get_object_or_404(Product, pk=pk)
```

## ModelForm

`ModelForm` auto-generates form fields from a model class. Use it when the form maps directly to a model.

```python
class ProductForm(forms.ModelForm):
    class Meta:
        model = Product
        fields = ["name", "price", "description"]
```

## Validation

Three levels:

1. `clean_<field>()` for field-level validation (runs after field's own validation)
2. `clean()` for cross-field validation (runs after all field cleaners succeed)
3. `Meta.constraints` for database-level enforcement (catches everything that bypasses forms)

```python
class OrderForm(forms.ModelForm):
    def clean_quantity(self):
        qty = self.cleaned_data["quantity"]
        if qty <= 0:
            raise forms.ValidationError("Quantity must be positive.")
        return qty

    def clean(self):
        cleaned = super().clean()
        start = cleaned.get("start_date")
        end = cleaned.get("end_date")
        if start and end and start >= end:
            raise forms.ValidationError("Start date must be before end date.")
        return cleaned
```

Always use `form.cleaned_data` after `is_valid()`. Never use `form.data` or `request.POST` directly for business logic.

## Redirect after POST

Never render a template in response to a POST. Redirect to a GET after successful form submission to prevent duplicate submissions on browser refresh.

## References

- https://docs.djangoproject.com/en/stable/topics/class-based-views/
- https://docs.djangoproject.com/en/stable/topics/forms/
- https://docs.djangoproject.com/en/stable/ref/class-based-views/
- https://docs.djangoproject.com/en/stable/topics/forms/modelforms/
