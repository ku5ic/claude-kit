# Design patterns

Which pattern a signal calls for. Apply one only where its signal is present, in the order and under the test `skills/kit-rules/reference/change.md` section 10 sets, as a template to adapt, never code to copy in.

| Signal                                                            | Pattern                                                                                         | Example                                                                            |
| ----------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| The same shape repeated with different data                       | Table-driven: the data as rows, one loop over them                                              | a validation block per form field -> a list of field rules and one validator       |
| A branch on kind or type in several places                        | Dispatch map or strategy: one lookup from kind to behavior                                      | `switch` on payment method in checkout, refund, and receipt -> one map             |
| Several readers of one source, each parsing it                    | One iterator that yields typed entries to every consumer                                        | three functions each opening and parsing the same log -> one reader                |
| The same multi-step sequence at many call sites                   | Extract one function, or a pipeline of steps                                                    | fetch, check status, parse JSON, map errors in every API call -> one client        |
| Wrappers that only forward, each with its own signature           | One interface or function type every implementation meets                                       | an adapter per handler only to match the router -> handlers share one shape        |
| One concept spread across many files                              | An owner module (facade) the rest call                                                          | date formatting rules repeated in each view -> one formatting module               |
| Many optional parameters, or setup copied before each use         | Builder or defaults struct                                                                      | a constructor called with the same five arguments everywhere -> one default        |
| Behavior added around existing calls in several places            | Decorator or middleware                                                                         | auth and logging copied into each endpoint -> one wrapper around them              |
| A third-party API's shape leaking into many call sites            | Adapter: one module maps it to the interface you need                                           | SDK response fields read across the app -> one adapter returning your types        |
| The same choice of which concrete type to build, repeated         | Factory: one function picks and builds it                                                       | `if env == "prod"` choosing a storage client in five places -> one factory         |
| Business logic calling a concrete database, clock, or HTTP client | Dependency inversion: the logic owns a small interface; the caller passes the implementation in | a service that needs a live database in tests -> an injected store, faked in tests |
| One change makes a module import and call several unrelated ones  | Observer or events: publish once, each module subscribes                                        | saving an order calls email, analytics, and stock code -> one `order.saved` event  |
| Status read from flag combinations in many places                 | State machine: one explicit state and a table of allowed transitions                            | `isLoading && !error && !done` across a form -> one `state` with allowed moves     |
| The same null or missing check before calls, repeated             | Null object, or an optional or default value                                                    | `if logger != nil` before every log call -> a no-op logger by default              |

- A language feature over a classic pattern: a map of functions over a strategy hierarchy, a generic over a template class, a closure over a command object.
- An interface for dependency inversion needs a second implementation, and a test double counts. One with neither is the single-implementation interface `skills/kit-rules/reference/change.md` section 8 rules out.
- A concurrency pattern (worker pool, pipeline, fan-out) only when the signal is a real wait or contention, never a guess about scale.
