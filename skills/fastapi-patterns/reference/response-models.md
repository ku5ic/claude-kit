# Response Models

## response_model

Declare `response_model` on the route decorator to filter and validate the response:

```python
from fastapi import FastAPI
from pydantic import BaseModel

class UserRead(BaseModel):
    id: int
    username: str
    email: str

class UserInDB(UserRead):
    hashed_password: str

@app.get("/users/{user_id}", response_model=UserRead)
def get_user(user_id: int, db: Annotated[Session, Depends(get_db)]):
    return db.query(User).filter(User.id == user_id).first()
```

FastAPI filters the return value to only the fields declared in `UserRead`. `hashed_password` is never serialized into the response even if the ORM object contains it.

`response_model` also generates the correct OpenAPI schema for the response shape, which drives the `/docs` UI.

## response_model_exclude_unset

Omit fields from the response when they were not explicitly set by the route handler. Useful for PATCH endpoints that should only return changed fields:

```python
@app.patch("/users/{user_id}", response_model=UserRead, response_model_exclude_unset=True)
def patch_user(user_id: int, update: UserUpdate, db: Annotated[Session, Depends(get_db)]):
    user = db.query(User).filter(User.id == user_id).first()
    update_data = update.model_dump(exclude_unset=True)
    for key, value in update_data.items():
        setattr(user, key, value)
    db.commit()
    db.refresh(user)
    return user
```

## Separate input and output schemas

When the write shape differs from the read shape, use separate schemas:

```python
class UserCreate(BaseModel):
    username: str
    email: str
    password: str          # accepted on input

class UserRead(BaseModel):
    model_config = ConfigDict(from_attributes=True)
    id: int
    username: str
    email: str             # password intentionally absent

@app.post("/users/", response_model=UserRead, status_code=201)
def create_user(user_in: UserCreate, db: Annotated[Session, Depends(get_db)]):
    hashed = hash_password(user_in.password)
    user = User(username=user_in.username, email=user_in.email, hashed_password=hashed)
    db.add(user)
    db.commit()
    db.refresh(user)
    return user
```

The password exclusion is enforced by the schema boundary, not by filtering logic in the handler.

## response_model vs return type annotation

Both `response_model=UserRead` on the decorator and `-> UserRead` on the function signature influence OpenAPI docs. They behave slightly differently: `response_model` applies FastAPI filtering and validation; the return annotation is used by type checkers. Using both is valid and recommended for consistent IDE support.

## Status codes

Set non-200 success codes on the decorator:

```python
@app.post("/items/", response_model=ItemRead, status_code=201)
@app.delete("/items/{item_id}", status_code=204)
```

Use `fastapi.status` constants (`status.HTTP_201_CREATED`, `status.HTTP_204_NO_CONTENT`) instead of bare integers for readability.

## References

- https://fastapi.tiangolo.com/tutorial/response-model/
