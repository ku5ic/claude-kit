---
name: monitoring-patterns
description: Prometheus patterns for prometheus-client metric types, naming, labels, and health checks. Use for Python metrics code or monitoring questions.
---

# Monitoring Patterns

Review checklist for Prometheus instrumentation. Constructor signatures are in the prometheus-client docs.

## Picking a metric type

| Type      | Use for                                                           | Goes down? |
| --------- | ----------------------------------------------------------------- | ---------- |
| Counter   | Requests, errors, bytes: totals that only increase                | No         |
| Gauge     | Queue depth, active connections, memory: current state            | Yes        |
| Histogram | Latency, payload size: distribution matters                       | No         |
| Summary   | Avoid; the Python client exposes only count and sum, no quantiles | No         |

- A distribution: use a Histogram, not a Summary.

## Naming

`namespace_subsystem_name_unit`, snake_case, base units only.

- A counter: end its name in `_total`, as in `myapp_errors_total`.
- Units: `_seconds` and `_bytes`, never `_milliseconds` or `_megabytes`.
- Name what's measured, not the measurement: `http_request_duration_seconds`, not `http_request_time`.

## Exposing

- A background worker or CLI with no HTTP server: call `start_http_server(8000)`.
- ASGI: mount `make_asgi_app()` at `/metrics`.
- WSGI: mount `make_wsgi_app()` at `/metrics`.
- Before upgrading prometheus-client across minor versions, read its GitHub release notes.

## Health endpoints

| Endpoint        | Returns 200 when                       | Used by                    |
| --------------- | -------------------------------------- | -------------------------- |
| `/health/live`  | Process is running                     | Kubernetes liveness probe  |
| `/health/ready` | Dependencies (DB, cache) are reachable | Kubernetes readiness probe |

- A dependency is down: return 503, not 500.
- Readiness: fail only when the instance can't serve traffic.

## Anti-patterns

- A label value with unbounded cardinality, such as a user ID, request ID, or email address: `failure`. Keep label values bounded and low-cardinality.
- A Counter for a value that can decrease, such as queue depth or active sessions: `failure`. Use a Gauge.
- A metric name in `_milliseconds`: `warning`. Convert to seconds and use `_seconds`.
- A metric with no namespace prefix: `warning`. Set `namespace="yourapp"`.
- One health endpoint for both liveness and readiness: `warning`. Keep them separate.
- The default Histogram buckets (.005 to 10, tuned for HTTP latency) taken without review: `info`. Define buckets that match the operation's real distribution.
