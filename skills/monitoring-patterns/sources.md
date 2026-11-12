# monitoring-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- https://prometheus.io/docs/concepts/metric_types/
- https://prometheus.github.io/client_python/
- https://prometheus.io/docs/practices/naming/

### Version notes

Checked: 2026-11-10 against https://github.com/prometheus/client_python/releases, https://prometheus.github.io/client_python/instrumenting/summary/, https://prometheus.io/download/, https://github.com/prometheus/prometheus/releases

- Prometheus 3.16.0 (2026-11-04), LTS 3.13.5; prometheus-client 0.26.0 still current: no guidance change.
- prometheus-client 0.26.0 (2026-07-24), Prometheus 3.15.0 (2026-09-24): Summary row corrected; the Python client "does not compute quantiles locally".

### Left to linters

Checked: 2026-11-11 with ruff 0.17.4 defaults (`uvx ruff check --isolated`, no config, no `--select`) on a probe file. No rule here was dropped: metric naming, labels, and health endpoints aren't Python lint targets.

Not caught by defaults, so kept: a `_milliseconds` or `_megabytes` metric name, a metric with no namespace, a user ID as a label value, and a Summary.
