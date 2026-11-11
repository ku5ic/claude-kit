# Errors

- Extracting an error type on 1.26+: `errors.AsType[*fs.PathError](err)`. Below 1.26: `errors.As`.
- A returned error: check it. Never discard it with `_`.
- Adding context: wrap with `%w`, as in `fmt.Errorf("load config %s: %w", path, err)`. Never `%v`.
- Matching a sentinel: `errors.Is(err, ErrNotFound)`, never `err == ErrNotFound`.
- Several errors to return as one: `errors.Join`.
- A normal failure: return an `error`. Never panic.
- Signaling failure: an extra `error` or `ok bool` return, never an in-band `-1`, `""`, or `nil`.
- The error path: handle it first and return early. No `else` after a branch that returns.
