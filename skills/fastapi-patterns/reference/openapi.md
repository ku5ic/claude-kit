# OpenAPI

- A license with a well-known SPDX identifier, on FastAPI 0.99.0+ (OpenAPI 3.1.0): `license_info={"name": "Apache 2.0", "identifier": "Apache-2.0"}`, not `url`.
- App metadata: pass `title`, `description`, `version`, `contact`, and `license_info` to `FastAPI(...)`.
- Tag descriptions: `FastAPI(openapi_tags=[{"name": ..., "description": ..., "externalDocs": {...}}])`, then `tags=["orders"]` on each route.
- Route docs: `summary` for the one-line text in the route list, `description` for longer text, `response_description` for the label on the 200 response block in Swagger UI. `include_in_schema=False` hides a route from the docs.
- Interactive docs: development only. In production, set `docs_url`, `redoc_url`, and `openapi_url` to `None`, for example when `os.getenv("ENV") == "production"`. `None` disables that endpoint; `openapi_url=None` also disables the schema behind Swagger and ReDoc.
- A gateway or proxy that expects a specific schema path: `FastAPI(openapi_url="/api/v1/openapi.json")`.
