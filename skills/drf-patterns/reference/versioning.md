# Versioning

## Schemes

- Enable one of the five built-in schemes with `DEFAULT_VERSIONING_CLASS` in `REST_FRAMEWORK`, next to `DEFAULT_VERSION`, `ALLOWED_VERSIONS`, and `VERSION_PARAM`.
- `URLPathVersioning` puts the version in the URL path (`/api/v1/orders/`), so URL patterns need a `version` keyword argument, as in `path("api/<str:version>/orders/", ...)`. It's the most common choice for public APIs.
- `NamespaceVersioning` takes the version from Django URL namespaces and looks the same as `URLPathVersioning` to clients. Use it in larger projects that already use URL namespaces.
- `AcceptHeaderVersioning` puts the version in the `Accept` header (`Accept: application/json; version=1.0`). The DRF docs consider it best practice for RESTful design; it's harder for clients to implement.
- `QueryParameterVersioning` puts the version in a query param (`?version=1`). It's simple but semantically weak.
- `HostNameVersioning` puts the version in the hostname (`v1.api.example.com`). It suits routing each version to a different backend and is hard in local development.

## Accessing the version

- `request.version` holds the version the scheme determined, or `None` when versioning isn't configured. Branch on it in `get_serializer_class()`, as in `if self.request.version == "v2": return OrderV2Serializer`.

## Practical guidance

- Pick `URLPathVersioning` or `NamespaceVersioning`, the most common choices.
- Set `ALLOWED_VERSIONS` explicitly. DRF returns 404 for a version not in the list.
