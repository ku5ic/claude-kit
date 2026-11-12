---
name: fastapi-patterns
description: FastAPI and Pydantic v2 patterns for Depends, response models, async, errors, and auth. Load before any edit to a file importing fastapi, however small, and for FastAPI questions.
---

# FastAPI patterns

Default assumption: FastAPI 0.162.x, Pydantic 2.14.x. Adapt advice to the versions in the project's `requirements.txt`, `pyproject.toml`, or lockfile.

Install: `pip install "fastapi[standard]"`

## Reference files

Loading this skill isn't enough. Before your next edit, read the file of every row that matches it. The rules live in those files, and this one only points at them.

| Before                                                                               | Read                                                         |
| ------------------------------------------------------------------------------------ | ------------------------------------------------------------ |
| Writing or changing a Pydantic schema, field, or validator                           | [pydantic-schemas.md](reference/pydantic-schemas.md)         |
| Writing a `Depends()` dependency, a `yield` cleanup, or a test override              | [dependency-injection.md](reference/dependency-injection.md) |
| Choosing `async def` or `def` for an endpoint, or calling blocking or CPU-bound code | [async-correctness.md](reference/async-correctness.md)       |
| Setting a `response_model` or a status code                                          | [response-models.md](reference/response-models.md)           |
| Raising `HTTPException` or writing an exception handler                              | [error-handling.md](reference/error-handling.md)             |
| Setting OpenAPI metadata, tags, or route docs                                        | [openapi.md](reference/openapi.md)                           |
| Adding authentication, JWT, password hashing, or an API key                          | [auth.md](reference/auth.md)                                 |
| Writing, changing, or reviewing FastAPI code                                         | [anti-patterns.md](reference/anti-patterns.md)               |
