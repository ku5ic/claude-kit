# Views and forms

## Class-based vs function-based views

- CBV or FBV is a project-level convention, not a quality signal. Both are valid.
- CRUD flows with standard shapes: CBVs (`ListView`, `DetailView`, `CreateView`, `UpdateView`, `DeleteView`). Less code, but a deep inheritance chain that's harder to trace.
- One-off endpoints, complex logic, or readability over code reduction: FBVs.
- Pick one style per area. Never mix both for equivalent endpoints.

## Views

- A view parses the request and shapes the response. Business logic goes in services or model methods.
- A single-object lookup: `get_object_or_404(Product, pk=pk)`, never a manual `DoesNotExist` catch.
- A successful POST: redirect to a GET, never render a template.

## Forms

- A form that maps directly to a model: `ModelForm`, which generates fields from `Meta.model` and `Meta.fields`.
- Field-level validation: `clean_<field>()`, which runs after the field's own validation.
- Cross-field validation: `clean()`, starting from `super().clean()`. It runs after all field cleaners succeed.
- Database-level enforcement: `Meta.constraints`, which catches everything that bypasses forms.
- After `is_valid()`, read `form.cleaned_data`. Never use `form.data` or `request.POST` directly for business logic.
