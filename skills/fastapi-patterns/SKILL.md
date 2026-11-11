---
name: fastapi-patterns
description: FastAPI and Pydantic v2 patterns for Depends, response models, async, errors, and auth. Use for fastapi in dependencies, files importing fastapi, or FastAPI questions.
---

# FastAPI patterns

Default assumption: FastAPI 0.162.x, Pydantic 2.14.x. Adapt advice to the versions in the project's `requirements.txt`, `pyproject.toml`, or lockfile.

Install: `pip install "fastapi[standard]"`

## Reference files

| File                                                         | Covers                                                                                          |
| ------------------------------------------------------------ | ----------------------------------------------------------------------------------------------- |
| [pydantic-schemas.md](reference/pydantic-schemas.md)         | `from_attributes`, `Field()`, `field_validator`, `model_validator`, `ConfigDict`, split schemas |
| [dependency-injection.md](reference/dependency-injection.md) | `Annotated` dependencies, `yield` cleanup, class dependencies, caching, test overrides          |
| [async-correctness.md](reference/async-correctness.md)       | `async def` vs `def`, blocking calls, CPU-bound work                                            |
| [response-models.md](reference/response-models.md)           | `response_model`, `response_model_exclude_unset`, split schemas, status codes                   |
| [error-handling.md](reference/error-handling.md)             | `HTTPException`, exception handlers, Starlette's `HTTPException`, `detail`, 422s                |
| [openapi.md](reference/openapi.md)                           | SPDX license, metadata, tags, route docs, disabling docs in production                          |
| [auth.md](reference/auth.md)                                 | `OAuth2PasswordBearer` and JWT, password hashing, `APIKeyHeader`, `HTTPBasic`, optional auth    |
| [anti-patterns.md](reference/anti-patterns.md)               | Ten review-time anti-patterns with severity calls                                               |
