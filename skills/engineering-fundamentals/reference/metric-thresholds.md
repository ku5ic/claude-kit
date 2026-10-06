# Metric thresholds

The one set of numbers. None is a hard limit; each is a `warning` signal, and they compound: a 60-line function with 9 branches inside a 700-line file is one finding, not three. A single breach is rarely a defect on its own.

| Metric        | Warn when                                                                                    |
| ------------- | -------------------------------------------------------------------------------------------- |
| Function size | over 50 lines without a clear single job                                                     |
| Nesting       | conditionals nested deeper than 3                                                            |
| Branches      | more than 7 distinct branches in one function                                                |
| Coupling      | a module importing more than ~10 internal modules, or a type with more than 7 public methods |
| Cohesion      | a type whose methods work on disjoint subsets of its fields (two jobs)                       |
| File size     | over 500 lines, unless genuinely one cohesive unit                                           |

If the project's static analysis already reports a metric (cyclomatic or cognitive complexity), its readings count as input alongside these.
