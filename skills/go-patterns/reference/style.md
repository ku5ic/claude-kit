# Style and newer features

## Features by version

Suggest one only when the `go` directive is at least its version.

| Since | Feature                                                                                                                       |
| ----- | ----------------------------------------------------------------------------------------------------------------------------- |
| 1.27  | Generic methods: a method declares its own type parameters. Interface methods can't, and a generic method can't implement one |
| 1.27  | `encoding/json` runs on v2. `GOEXPERIMENT=nojsonv2` restores v1 until the opt-out is removed                                  |
| 1.27  | `uuid` package in the standard library                                                                                        |
| 1.26  | `new(expr)` allocates and initializes in one step: `Age: new(yearsSince(born))`                                               |
| 1.26  | `go fix` applies modernizers. Run it when raising the `go` directive                                                          |
| 1.22  | `for i := range 10` ranges over an integer                                                                                    |
| 1.22  | `http.ServeMux` patterns take a method and wildcards: `"POST /items/{id}"`, `r.PathValue("id")`, `{path...}` for the rest     |

## Code Review Comments

- An interface: declare it in the package that uses it. A producer returning its own interface for one implementation is a finding.
- Receiver names: a one or two letter abbreviation of the type, the same on every method.
- Initialisms: one case throughout, `userID` and `ServeHTTP`, never `userId`.
- Identifiers: don't repeat the package name, `chubby.File`, not `chubby.ChubbyFile`.
- Named results: only when they help the godoc, never to enable naked returns.
