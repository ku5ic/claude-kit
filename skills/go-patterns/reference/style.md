# Style and newer features

## Code Review Comments

- **Interfaces** generally belong in the package that uses values of the interface type, not the package that implements them. A producer package returning its own interface for one implementation is a finding.
- **Receiver names** reflect the type's identity, often a one or two letter abbreviation. Not `this` or `self`. Keep them consistent across a type's methods.
- **Initialisms** keep a consistent case: `URL`, `ID`, `HTTP`, so `userID` and `ServeHTTP`, never `userId`.
- **Package names**: callers refer to everything through the package name, so don't repeat it in identifiers (`chubby.File`, not `chubby.ChubbyFile`).
- **Named result parameters**: name them when it helps the godoc, not to enable naked returns. Clarity of the docs wins.

## Newer language and stdlib features

Only suggest one when the module's `go` directive is at least that version.

| Since | Feature                                                                                                                                 |
| ----- | --------------------------------------------------------------------------------------------------------------------------------------- |
| 1.22  | `for i := range 10` ranges over an integer                                                                                              |
| 1.22  | `http.ServeMux` patterns take a method and wildcards: `"POST /items/{id}"`, read with `r.PathValue("id")`; `{path...}` matches the rest |
| 1.26  | `new(expr)` allocates and initializes in one step, handy for optional pointer fields: `Age: new(yearsSince(born))`                      |
| 1.27  | Generic methods: a method can declare its own type parameters. Interface methods can't, and a generic method can't implement one        |
| 1.27  | `encoding/json` runs on the v2 implementation; `GOEXPERIMENT=nojsonv2` restores v1, and that opt-out is slated for removal              |
| 1.27  | New `uuid` package in the standard library                                                                                              |

`go fix` (rebuilt in 1.26) applies modernizers built on the same analysis framework as `go vet`; run it when raising the `go` directive.
