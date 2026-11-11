# drf-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- https://www.django-rest-framework.org/
- https://www.django-rest-framework.org/api-guide/serializers/
- https://www.django-rest-framework.org/api-guide/permissions/
- https://django-rest-framework-simplejwt.readthedocs.io/en/latest/

### Version notes

Checked: 2026-11-10 against https://pypi.org/project/djangorestframework/, https://www.django-rest-framework.org/community/release-notes/ and https://www.django-rest-framework.org/api-guide/permissions/

- 3.18.3 (2026-10-06): current. Corrected permission composition (`&`, `|`, `~` are built in) and the simplejwt wording to match the docs.
- 3.18.1 (2026-09-07): 3.18.0 dropped Django 4.2, 5.0, 5.1. A project pinned to one of those stays on 3.17.
- 3.14: `NullBooleanField` removed; use `BooleanField(allow_null=True)`.

### Claims

- Django REST Framework patterns, DRF 3.18.x. Supports Django 5.2, 6.0, and 6.1; 3.18.0 dro...: https://pypi.org/project/djangorestframework/

## reference/auth.md

### References

- https://www.django-rest-framework.org/api-guide/authentication/
- https://django-rest-framework-simplejwt.readthedocs.io/en/latest/

## reference/filtering.md

### References

- https://www.django-rest-framework.org/api-guide/filtering/
- https://django-filter.readthedocs.io/en/stable/

## reference/permissions.md

### References

- https://www.django-rest-framework.org/api-guide/permissions/
- https://www.django-rest-framework.org/api-guide/permissions/#custom-permissions

## reference/serializers.md

### References

- https://www.django-rest-framework.org/api-guide/serializers/
- https://www.django-rest-framework.org/api-guide/serializers/#modelserializer
- https://www.django-rest-framework.org/api-guide/fields/#serializermethodfield

## reference/throttling-and-pagination.md

### References

- https://www.django-rest-framework.org/api-guide/throttling/
- https://www.django-rest-framework.org/api-guide/pagination/

## reference/versioning.md

### References

- https://www.django-rest-framework.org/api-guide/versioning/

## reference/viewsets-and-routers.md

### References

- https://www.django-rest-framework.org/api-guide/viewsets/
- https://www.django-rest-framework.org/api-guide/routers/
- https://www.django-rest-framework.org/api-guide/generic-views/
