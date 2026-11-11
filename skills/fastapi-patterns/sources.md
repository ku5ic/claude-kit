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

- FastAPI patterns, FastAPI 0.162.x, Pydantic 2.14.x (baseline at time of wri...: https://pypi.org/project/fastapi/, https://pypi.org/project/pydantic/

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
