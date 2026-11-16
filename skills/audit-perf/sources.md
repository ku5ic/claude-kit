# audit-perf sources

The sources and version notes behind this skill, for `/meta-refresh`. Nothing links here, so a session never loads it.

## reference/measuring.md

### References

- Core Web Vitals: https://web.dev/articles/vitals
- CLS causes: https://web.dev/articles/cls
- PostgreSQL indexes: https://www.postgresql.org/docs/current/indexes-intro.html
- hyperfine: https://github.com/sharkdp/hyperfine

### Version notes

Checked: 2026-11-16 against web.dev, the PostgreSQL docs, and the hyperfine README.

## reference/runtime.md

### References

- Lab and field data: https://web.dev/articles/lab-and-field-data-differences
- Lighthouse README: https://github.com/GoogleChrome/lighthouse/blob/main/readme.md
- Lighthouse variability: https://github.com/GoogleChrome/lighthouse/blob/main/docs/variability.md
- Lighthouse CI: https://github.com/GoogleChrome/lighthouse-ci/blob/main/docs/getting-started.md
- DevTools Performance panel: https://developer.chrome.com/docs/devtools/performance/reference
- benchstat: https://pkg.go.dev/golang.org/x/perf/cmd/benchstat
- Go diagnostics: https://go.dev/doc/diagnostics
- Postgres EXPLAIN: https://www.postgresql.org/docs/current/sql-explain.html
- MySQL EXPLAIN: https://dev.mysql.com/doc/refman/8.4/en/explain.html
- MySQL slow query log: https://dev.mysql.com/doc/refman/8.4/en/slow-query-log.html
- Postgres logging: https://www.postgresql.org/docs/current/runtime-config-logging.html
- k6 test types: https://grafana.com/docs/k6/latest/testing-guides/test-types/
- wrk: https://github.com/wg/wrk

### Version notes

Checked: 2026-11-16 against each source. Postgres 18 and MySQL 8.4 docs.
