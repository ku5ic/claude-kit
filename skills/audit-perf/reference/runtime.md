# Runtime

- The project's own tools first, meaning a benchmark script, a profiler, or a perf task in its manifests, task runner, or CI, then one already on this machine. An install isn't offered.
- Lab data comes from a controlled run and fits debugging. Field data comes from real users and sets priority. A lab number confirms a finding, and field data says whether it matters.
- Lighthouse throttles network and CPU by default, so its numbers don't compare with an unthrottled DevTools trace.
- Lighthouse scores vary run to run. The median of 5 runs is twice as stable as 1. Never run two at once on one machine.
- `lighthouse <url> --output=json --only-categories=performance` gives a machine-readable run. A repo's Lighthouse CI `assert` config holds the project's own budgets.
- The DevTools Performance panel throttles relative to the local machine and can't truly simulate a mobile CPU.
- A profile shows where time goes, a benchmark how long. A finding that quotes a profile names its top frames.
- Compare distributions, never single numbers. `benchstat` wants at least 10 runs, ideally 20, a count picked up front and kept.
- Benchmarks run on an otherwise idle machine. A result with many outliers isn't evidence.
- One profile at a time, since profiling skews what it measures.
- Postgres `EXPLAIN ANALYZE` executes the statement. A write isn't offered: even inside `BEGIN; ... ROLLBACK;` it takes locks on whatever database the project points at. `TIMING FALSE` drops the per-node clock overhead.
- MySQL `EXPLAIN ANALYZE` also runs the statement and reports actual time and rows per step.
- Slow-query logging is off by default in MySQL and Postgres. Turning it on is a config change on someone's database, so it isn't offered. Read an existing log.
- A load tool (`k6 run`, `wrk`) drives real traffic, so it's offered only against a server on this machine, starting with a smoke test at minimal load. A shared target or production isn't offered.
- Report what ran, with the command, the run count, the median and spread, and the machine state.
