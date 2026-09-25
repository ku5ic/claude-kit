# Error Handling

## HTTPException

Raise `HTTPException` to return an error response. Do not return it.

```python
from fastapi import FastAPI, HTTPException, status

@app.get("/items/{item_id}")
def get_item(item_id: int, db: Annotated[Session, Depends(get_db)]):
    item = db.query(Item).filter(Item.id == item_id).first()
    if item is None:
        raise HTTPException(status_code=404, detail="Item not found")
    return item
```

Use `fastapi.status` constants for readability:

```python
raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Item not found")
raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail="Not authorized")
raise HTTPException(
    status_code=status.HTTP_401_UNAUTHORIZED,
    detail="Could not validate credentials",
    headers={"WWW-Authenticate": "Bearer"},
)
```

The `headers` parameter on `HTTPException` adds response headers to the error response.

## Custom exception handlers

Register handlers for application-specific exceptions instead of catching them in every route:

```python
class InsufficientFundsError(Exception):
    def __init__(self, balance: float, required: float):
        self.balance = balance
        self.required = required

@app.exception_handler(InsufficientFundsError)
async def insufficient_funds_handler(request: Request, exc: InsufficientFundsError):
    return JSONResponse(
        status_code=422,
        content={"detail": f"Balance {exc.balance} insufficient; need {exc.required}"},
    )
```

Routes can now raise `InsufficientFundsError` directly without catching it.

## Starlette vs FastAPI HTTPException

FastAPI's `HTTPException` (from `fastapi`) and Starlette's `HTTPException` (from `starlette.exceptions`) are different classes. Handlers registered with `@app.exception_handler(HTTPException)` only catch FastAPI's version. To also catch Starlette exceptions (raised by middleware and routing internals), register a handler for `starlette.exceptions.HTTPException` separately, or override the default handler.

## Never expose internals in detail

The `detail` field is serialized directly into the response body. Do not put:

- stack traces
- database error messages
- internal paths or file names
- raw exception messages from third-party libraries

Log the internal error, return a generic message to the client:

```python
except Exception as exc:
    logger.exception("Unexpected error processing payment", exc_info=exc)
    raise HTTPException(status_code=500, detail="Payment processing failed")
```

## Validation errors

FastAPI automatically returns 422 Unprocessable Entity for Pydantic validation failures. The response body is a structured JSON with per-field error locations and messages. This is the default; no code needed. If you need to customize the 422 response shape, override the `RequestValidationError` handler from `fastapi.exceptions`.

## References

- https://fastapi.tiangolo.com/tutorial/handling-errors/
