---
name: monitoring-patterns
description: >
  Application monitoring patterns covering Prometheus metrics (Counter, Gauge,
  Histogram, Summary), the prometheus-client Python library, metric naming
  conventions, labels, and health check endpoints. Use whenever a Python project
  instruments metrics, uses prometheus-client, or the user asks about Prometheus,
  metrics, monitoring, health checks, or observability, even if "Prometheus" is
  not mentioned by name.
---

# Monitoring Patterns

Review checklist for Prometheus instrumentation. Constructor signatures are in the prometheus-client docs; this file is the naming rules, the cardinality trap, and what to flag.

## Picking a metric type

| Type      | Use for                                                                             | Goes down? |
| --------- | ----------------------------------------------------------------------------------- | ---------- |
| Counter   | Requests, errors, bytes - totals that only increase                                 | No         |
| Gauge     | Queue depth, active connections, memory - current state                             | Yes        |
| Histogram | Latency, payload size - distribution matters                                        | No         |
| Summary   | Avoid; quantiles are computed client-side and cannot be aggregated across instances | No         |

Default to Histogram over Summary. Review the default buckets (.005 through 10, tuned for HTTP latency) against the operation's real distribution rather than accepting them.

## Naming

`namespace_subsystem_name_unit`, snake_case, base units only.

- Counters end in `_total`: `myapp_errors_total`
- `_seconds` not `_milliseconds`, `_bytes` not `_megabytes`
- Name what is measured, not the measurement: `http_request_duration_seconds`, not `http_request_time`
- Always set `namespace=` - unprefixed names collide across apps in a shared Prometheus

## Exposing

- Background worker or CLI with no HTTP server: `start_http_server(8000)`
- ASGI: mount `make_asgi_app()` at `/metrics`
- WSGI: mount `make_wsgi_app()` at `/metrics`

## Health endpoints

| Endpoint        | Returns 200 when                       | Used by                    |
| --------------- | -------------------------------------- | -------------------------- |
| `/health/live`  | Process is running                     | Kubernetes liveness probe  |
| `/health/ready` | Dependencies (DB, cache) are reachable | Kubernetes readiness probe |

Return 503, not 500, when a dependency is down - readiness should fail only when the instance cannot serve traffic.

---

## Anti-patterns

**failure: high-cardinality label values**
Using user IDs, request IDs, or email addresses as label values creates millions of time series, crashes Prometheus, and makes dashboards useless. Labels must have bounded, low cardinality.

**failure: Counter used for values that decrease**
Use a Gauge for queue depth, active sessions, or any value that can drop. A Counter that decreases corrupts `rate()` calculations in PromQL.

**warning: metric names in milliseconds**
Prometheus convention is seconds. `_milliseconds` metrics are incompatible with standard recording rules and dashboards. Convert to seconds and use `_seconds`.

**warning: no namespace on metrics**
Without a namespace prefix, metric names from different apps collide in shared Prometheus instances. Always set `namespace="yourapp"`.

**warning: single health endpoint used for both liveness and readiness**
Combining them means a slow database causes the container to be killed and restarted rather than just pulled from load balancing. Keep them separate.

**info: default Histogram buckets accepted without review**
The defaults suit general HTTP latency. For faster or slower operations, define buckets that match the expected distribution.

---

## References

- https://prometheus.io/docs/concepts/metric_types/
- https://prometheus.github.io/client_python/
- https://prometheus.io/docs/practices/naming/

> prometheus-client's API surface is stable but check the GitHub release notes before upgrading across minor versions.
