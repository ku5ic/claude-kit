# fastapi-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- https://fastapi.tiangolo.com/
- https://pydantic.dev/docs/validation/latest/
- https://fastapi.tiangolo.com/tutorial/security/
- https://fastapi.tiangolo.com/async/

### Version notes

Checked: 2026-11-10 against https://pypi.org/project/fastapi/, https://pypi.org/project/pydantic/ and https://fastapi.tiangolo.com/release-notes/

- 0.162.4 (2026-11-10) with Pydantic 2.14.3 (2026-11-05): no change to the patterns here. Since 0.141: `ORJSONResponse` and `UJSONResponse` deprecated (0.131), `strict_content_type` on by default (0.132), the `root_path` parameter deprecated (0.146), and `**extra` removed from `Path`, `Query` and friends in favor of `json_schema_extra`.
- 0.141.1 (2026-07-29) with Pydantic 2.13.5 (2026-08-28): no guidance change. FastAPI stays on 0.x by design; there is no 1.x and no Pydantic 3.
- 0.128.0: Pydantic v1 compatibility (`pydantic.v1` models) removed; unsupported on Python 3.14+.

### Claims

- Default assumption FastAPI 0.162.x, Pydantic 2.14.x: https://pypi.org/project/fastapi/, https://pypi.org/project/pydantic/

### Left to linters

Checked: 2026-11-11 with ruff 0.17.4 defaults (`uvx ruff check --isolated`, no config, no `--select`) on a probe package. No `FAST` rule is in the default set, and no rule was dropped.

ruff `B008` reports `param: T = Depends(fn)`, but its suggested fix (call it inside the function, or use a module-level default) is wrong for FastAPI, so the `Annotated[T, Depends(fn)]` rule stays.

Not caught by defaults, so kept: a sync ORM call in an `async def` route (`ASYNC210` and `ASYNC251` catch `requests` and `time.sleep`, not a sync DB driver), `return HTTPException(...)`, `detail=str(exc)`, Pydantic v1 `class Config` and `@validator`, bare integer status codes (`PLR2004` is off), `APIKeyQuery`, credentials compared with `==`, `dependency_overrides` never reset, a route with no `response_model`, docs left on in production, CPU-bound work in `async def`.

## reference/async-correctness.md

### References

- https://fastapi.tiangolo.com/async/

## reference/auth.md

### References

- https://fastapi.tiangolo.com/tutorial/security/oauth2-jwt/
- https://fastapi.tiangolo.com/advanced/security/http-basic-auth/
- https://fastapi.tiangolo.com/reference/security/

## reference/dependency-injection.md

### References

- https://fastapi.tiangolo.com/tutorial/dependencies/
- https://fastapi.tiangolo.com/tutorial/dependencies/dependencies-with-yield/
- https://fastapi.tiangolo.com/advanced/testing-dependencies/

## reference/error-handling.md

### References

- https://fastapi.tiangolo.com/tutorial/handling-errors/

## reference/openapi.md

### References

- https://fastapi.tiangolo.com/tutorial/metadata/

## reference/pydantic-schemas.md

### References

- https://pydantic.dev/docs/validation/latest/
- https://pydantic.dev/docs/validation/latest/concepts/validators/
- https://pydantic.dev/docs/validation/latest/concepts/fields/

## reference/response-models.md

### References

- https://fastapi.tiangolo.com/tutorial/response-model/
