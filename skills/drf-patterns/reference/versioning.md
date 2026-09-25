# Versioning

## Schemes

DRF provides five built-in versioning schemes. Enable one via `DEFAULT_VERSIONING_CLASS`:

```python
REST_FRAMEWORK = {
    "DEFAULT_VERSIONING_CLASS": "rest_framework.versioning.URLPathVersioning",
    "DEFAULT_VERSION": "v1",
    "ALLOWED_VERSIONS": ["v1", "v2"],
    "VERSION_PARAM": "version",
}
```

**URLPathVersioning**: version in the URL path (`/api/v1/orders/`). Requires URL patterns with a `version` keyword argument. Most common choice for public APIs.

```python
urlpatterns = [
    path("api/<str:version>/orders/", OrderViewSet.as_view({"get": "list"})),
]
```

**NamespaceVersioning**: version via Django URL namespaces. Looks identical to URLPathVersioning from the client. Cleaner for larger projects that already use URL namespaces.

**AcceptHeaderVersioning**: version in the `Accept` header (`Accept: application/json; version=1.0`). Considered best practice for RESTful design by DRF docs because the URL stays stable. More complex for clients to implement.

**QueryParameterVersioning**: version as a query param (`?version=1`). Simple but semantically weak.

**HostNameVersioning**: version in the hostname (`v1.api.example.com`). Useful for routing to different backends per version. Difficult in local development.

## Accessing the version

`request.version` returns the version string determined by the scheme, or `None` if versioning is not configured.

```python
class OrderViewSet(viewsets.ModelViewSet):
    def get_serializer_class(self):
        if self.request.version == "v2":
            return OrderV2Serializer
        return OrderV1Serializer
```

## Practical guidance

URLPathVersioning or NamespaceVersioning are the most common choices. URL-based versioning is easy to document, test with curl, and visible in logs.

Set `ALLOWED_VERSIONS` explicitly. DRF returns 404 for versions not in the list, preventing clients from reaching unintended code paths.

## References

- https://www.django-rest-framework.org/api-guide/versioning/
