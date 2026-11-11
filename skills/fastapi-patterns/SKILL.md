---
name: fastapi-patterns
description: FastAPI and Pydantic v2 patterns for Depends, response models, async, errors, and auth. Use for fastapi in dependencies, files importing fastapi, or FastAPI questions.
---

# FastAPI patterns

FastAPI 0.162.x, Pydantic 2.14.x (baseline at time of writing). Adapt advice to the versions in the project's `requirements.txt`, `pyproject.toml`, or lockfile.

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
