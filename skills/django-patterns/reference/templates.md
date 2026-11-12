# Templates

## Inheritance

- Layout: `{% extends %}` with `{% block %}`. A base template defines structural blocks and child templates fill them.
- Reusable partials without structural meaning: `{% include %}`. Never use it for structural layout pieces.

## Auto-escaping and CSRF

- Templates auto-escape every variable by default (`<`, `>`, `"`, `'`, `&`).
- `|safe`, `{% autoescape off %}`, or `mark_safe()` on user-controlled content: `failure` (XSS). Output it as plain `{{ comment }}`.
- Every POST form: `{% csrf_token %}` inside it. Django's form rendering doesn't add it.
- `@csrf_exempt` on a view that mutates state: `failure`, unless the endpoint authenticates by API token, not session. Then document why next to the decorator.

## Logic in templates

- `{% if %}` nesting deeper than 3-4 levels: move the logic to the view context or a template tag.
- Context processors: only for globally available data. View-specific data goes in the view's context explicitly.
