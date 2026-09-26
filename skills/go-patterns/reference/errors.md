# Errors

## Check every error

Don't discard an error with `_`. If a function returns an error, check it. Source: https://go.dev/wiki/CodeReviewComments#handle-errors

## Wrap with `%w`, inspect with `errors`

- Add context while keeping the cause inspectable: `fmt.Errorf("load config %s: %w", path, err)`. `%v` flattens the chain, so callers can no longer match the cause.
- Compare with `errors.Is(err, ErrNotFound)`, not `err == ErrNotFound`, which misses wrapped errors.
- Extract a type with `errors.As`, or with `errors.AsType` on Go 1.26+. The docs call `AsType` type-safe, faster, and in most cases easier to use than `As`.
- `errors.Join` combines several errors into one.

Sources: https://pkg.go.dev/errors, https://go.dev/doc/go1.26

## Error strings

Lowercase, unless the first word is a proper noun or acronym, and no trailing punctuation. They usually end up in the middle of another message. Source: https://go.dev/wiki/CodeReviewComments#error-strings

## Don't panic for normal errors

Use an `error` return and multiple return values. Source: https://go.dev/wiki/CodeReviewComments#dont-panic

## No in-band errors

Don't signal failure with a special value like `-1`, `""`, or `nil`. Return an extra value instead: an `error`, or an `ok bool`. Source: https://go.dev/wiki/CodeReviewComments#in-band-errors

## Indent the error path

Handle the error first and return early, keeping the normal path at minimal indentation. No `else` after a branch that returns. Source: https://go.dev/wiki/CodeReviewComments#indent-error-flow
