# Dependencies

Timeouts, retries, and breakers are `/audit-arch`'s. This file holds the rest.

- List every third-party service, data source, or event the launch depends on, and what happens if its vendor misses the launch.
- Each dependency gets a failure model, its soft and hard failure modes and the impact of each on the service and on customers.
- Load shedding drops some load as the server nears overload. Look for an in-flight cap that rejects early, and a throttle before expensive work.
- A graceful-degradation path that never runs is a path that often doesn't work. Ask for a test or a drill that hits it.
- Load test until things break, before launch. No result on file is the finding, not a missing tool.
