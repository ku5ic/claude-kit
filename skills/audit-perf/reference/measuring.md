# Measuring

- Core Web Vitals targets, at the 75th percentile of page loads: LCP 2.5 s, INP 200 ms, CLS 0.1. A static review can't read them, so a finding names the one it would move and says to measure it.
- Media without dimensions is a named cause of layout shift.
- An index helps a column in a `WHERE` or a `JOIN`, and costs every write. A missing-index finding names the query and the column; an index nothing uses is a finding too.
- The planner may ignore an index. Confirming one helps takes `EXPLAIN`, which this audit doesn't run, so the finding says to measure with it.
- `hyperfine` runs a command at least 10 times. `--warmup` fits a warm-cache case, `--prepare` resets state before each run for a cold one. Report the run count and the spread, never one number.
- A hyperfine result with outlier warnings isn't evidence of a difference.
