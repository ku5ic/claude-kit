# Metric thresholds

The one set of numbers.

- Each breach is a `warning` signal, never a hard limit. A single breach is rarely a defect on its own.
- Breaches compound into one finding: a 60-line function with 9 branches inside a 700-line file is one finding, not three.

| Metric        | Warn when                                                                                    |
| ------------- | -------------------------------------------------------------------------------------------- |
| Function size | over 50 lines without a clear single job                                                     |
| Nesting       | conditionals nested deeper than 3                                                            |
| Branches      | more than 7 distinct branches in one function                                                |
| Coupling      | a module importing more than ~10 internal modules, or a type with more than 7 public methods |
| Cohesion      | a type whose methods work on disjoint subsets of its fields (two jobs)                       |
| File size     | over 500 lines, unless genuinely one cohesive unit                                           |
| Step size     | a plan step naming more than 10 files: more than one review sheet holds                      |

- The project's static analysis already reports a metric (cyclomatic or cognitive complexity): its readings count as input alongside these.
