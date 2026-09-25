---
name: fastapi-patterns
description: FastAPI patterns, Pydantic schemas, dependency injection, async correctness, response models, error handling, OpenAPI, and auth. Use whenever the project contains `fastapi` in dependencies, files importing from `fastapi`, `@app.get`/`@router.get` decorators, Pydantic BaseModel subclasses used as request/response types, OR the user asks about FastAPI, Pydantic v2, Depends(), HTTPException, OAuth2PasswordBearer, APIKeyHeader, response_model, even if FastAPI is not mentioned by name.
---

# FastAPI patterns

FastAPI 0.141.x, Pydantic 2.13.x (baseline at time of writing). Verify current versions at https://pypi.org/project/fastapi/ and https://pypi.org/project/pydantic/. Adapt advice to the versions in the project's `requirements.txt`, `pyproject.toml`, or lockfile.

Install: `pip install "fastapi[standard]"`

## Reference files

| File                                                         | Covers                                                                                                        |
| ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------- |
| [pydantic-schemas.md](reference/pydantic-schemas.md)         | BaseModel, Field(), field_validator, model_validator, model_config, ConfigDict, separate input/output schemas |
| [dependency-injection.md](reference/dependency-injection.md) | Depends(), Annotated pattern, yield dependencies, testing overrides                                           |
| [async-correctness.md](reference/async-correctness.md)       | async def vs def, threadpool behavior, blocking I/O rules                                                     |
| [response-models.md](reference/response-models.md)           | response_model, response_model_exclude_unset, separate schemas                                                |
| [error-handling.md](reference/error-handling.md)             | HTTPException, custom exception handlers, status codes                                                        |
| [openapi.md](reference/openapi.md)                           | Metadata, tags, disabling docs in production, route decorator params                                          |
| [auth.md](reference/auth.md)                                 | OAuth2PasswordBearer, JWT, APIKeyHeader, HTTPBasic, timing attacks                                            |
| [anti-patterns.md](reference/anti-patterns.md)               | Severity-labeled anti-patterns to flag in review                                                              |

## References

- https://fastapi.tiangolo.com/
- https://pydantic.dev/docs/validation/latest/
- https://fastapi.tiangolo.com/tutorial/security/
- https://fastapi.tiangolo.com/async/

## Version notes

Checked: 2026-09-12 against https://pypi.org/project/fastapi/ and https://fastapi.tiangolo.com/release-notes/

- 0.141.1 (2026-07-29) with Pydantic 2.13.5 (2026-08-28): no guidance change. FastAPI stays on 0.x by design; there is no 1.x and no Pydantic 3.
- 0.128.0: Pydantic v1 compatibility (`pydantic.v1` models) removed; unsupported on Python 3.14+.
- 0.95.0: `Annotated` form for `Depends()` became the recommended shape.
