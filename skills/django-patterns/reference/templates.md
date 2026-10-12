# Templates

## Inheritance

Use `{% block %}` and `{% extends %}` for template inheritance. A base template defines structural blocks; child templates fill them.

```html
{# base.html #}
<!DOCTYPE html>
<html>
  <body>
    <main>{% block content %}{% endblock %}</main>
  </body>
</html>
```

```html
{# product_detail.html #} {% extends "base.html" %} {% block content %}
<h1>{{ product.name }}</h1>
{% endblock %}
```

Use `{% include %}` for reusable partials (fragments without structural meaning). Do not use it for structural layout pieces; use `{% extends %}` instead.

## Auto-escaping

Django templates auto-escape all variable output by default. Characters like `<`, `>`, `"`, `'`, and `&` are converted to their HTML entity equivalents.

The `|safe` filter, `{% autoescape off %}`, and `mark_safe()` in Python disable this escaping. Each is a `failure` when applied to user-controlled content. The Django security docs explicitly call this out as an XSS vector.

```html
{# WRONG: XSS if comment is user input #} {{ comment|safe }} {# Correct:
auto-escape handles it #} {{ comment }}
```

## CSRF

`{% csrf_token %}` is required in every POST form. It is not added automatically by Django's form rendering. Omitting it will cause 403 errors in production.

```html
<form method="post">
  {% csrf_token %} {{ form.as_p }}
  <button type="submit">Submit</button>
</form>
```

`@csrf_exempt` on a view that mutates state is a `failure` unless the endpoint authenticates by API token rather than session; document why next to the decorator.

## Logic in templates

Keep conditional logic minimal. A chain of `{% if %}` deeper than 3-4 levels is a sign that the view context or a template tag should handle the logic instead.

Context processors for globally available data. Everything else goes in the view's context explicitly. Avoid implicit reliance on context processors for view-specific data.

## References

- https://docs.djangoproject.com/en/stable/topics/templates/
- https://docs.djangoproject.com/en/stable/topics/security/ (XSS via |safe)
- https://docs.djangoproject.com/en/stable/ref/csrf/
