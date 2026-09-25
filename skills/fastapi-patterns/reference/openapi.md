# OpenAPI

## App-level metadata

Pass metadata when constructing the `FastAPI` instance:

```python
app = FastAPI(
    title="Orders API",
    description="Manages order lifecycle for the storefront.",
    version="1.2.0",
    contact={"name": "Platform team", "email": "platform@example.com"},
    license_info={"name": "Apache 2.0", "identifier": "Apache-2.0"},
)
```

`identifier` (SPDX format) is supported since FastAPI 0.99.0 / OpenAPI 3.1.0. Use instead of `url` when the license is a well-known SPDX identifier.

## Tag metadata

Group routes by tag and add descriptions to the tag group:

```python
tags_metadata = [
    {
        "name": "orders",
        "description": "Create, read, update, and cancel orders.",
    },
    {
        "name": "payments",
        "description": "Payment processing and refunds.",
        "externalDocs": {
            "description": "Payment provider docs",
            "url": "https://docs.provider.example.com/",
        },
    },
]

app = FastAPI(openapi_tags=tags_metadata)

@app.get("/orders/", tags=["orders"])
def list_orders(): ...

@app.post("/payments/", tags=["payments"])
def create_payment(): ...
```

## Route-level OpenAPI hints

```python
@app.get(
    "/items/{item_id}",
    summary="Get a single item",
    description="Returns the item with the given ID. Returns 404 if not found.",
    response_description="The requested item",
    tags=["items"],
    include_in_schema=True,  # set False to hide from docs
)
def get_item(item_id: int): ...
```

Use `summary` for the one-line description in the route list. Use `description` for longer explanation. `response_description` labels the 200 response block in Swagger UI.

## Disabling the schema in production

Expose the interactive docs only in development. In production, the API root and docs URLs reveal implementation details to attackers.

```python
import os

app = FastAPI(
    openapi_url="/openapi.json" if os.getenv("ENV") != "production" else None,
    docs_url="/docs" if os.getenv("ENV") != "production" else None,
    redoc_url="/redoc" if os.getenv("ENV") != "production" else None,
)
```

Setting any of these to `None` disables that endpoint entirely. `openapi_url=None` also disables the schema that backs Swagger and ReDoc.

## Custom OpenAPI URL

Rename the schema endpoint (useful when your API gateway or proxy expects a specific path):

```python
app = FastAPI(openapi_url="/api/v1/openapi.json")
```

## References

- https://fastapi.tiangolo.com/tutorial/metadata/
