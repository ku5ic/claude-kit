# Failure isolation

- An external call on a request path needs a timeout. Without one it holds a thread or a connection until the dependency answers.
- A timeout that's too long ties up resources across concurrent requests, nearly as badly as none. Short enough to fail fast, long enough to usually succeed.
- A retry has an upper bound and a growing delay, so many instances don't hammer a struggling service.
- Retry only an idempotent operation, or one carrying an idempotency key.
- Retries at two stacked layers multiply the delay. The lower layer fails fast and reports.
- A fault that isn't transient, such as a logic error or a 4xx, isn't retried.
- After repeated failures, stop calling for a while and return a fallback or a cached answer.
