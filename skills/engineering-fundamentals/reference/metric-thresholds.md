# Metric thresholds

Statically inferable smells. None are hard limits; each is a signal that compounds with other findings.

- **Function size:** over 50 lines without a clear cohesive reason. `warning`.
- **Cyclomatic complexity:** nested conditionals deeper than 3, or more than 7 distinct branches in one function. `warning`.
- **Coupling:** a module with more than ~10 internal imports, or a class with more than 7 public methods. `warning`.
- **Cohesion:** a class or module whose methods operate on disjoint subsets of fields suggests two responsibilities. `warning`.
- **File size:** over 500 lines is a candidate for split unless genuinely cohesive (single config, single component with tight internal cohesion).

These compound. A 60-line function with cyclomatic complexity 9 inside a 700-line file is one finding, not three.

## Calibrating against the literature

The function-size, complexity, and import thresholds in this skill are author judgment, calibrated against the broader literature.

- **McCabe (1976) recommended cyclomatic complexity below 10 per routine** ("split them into smaller modules whenever the cyclomatic complexity of the module exceeded 10"). NIST's Structured Testing methodology adopted the threshold of 10 and noted values up to 15 may be appropriate in some circumstances. The skill body's `warning` at > 7 is tighter than McCabe's "split" line on the principle that a warning should fire before the module is in genuine trouble.
- **Cohesion measurement** has a formal lineage in the LCOM (Lack of Cohesion of Methods) family of metrics, building on Stevens / Myers / Constantine's cohesion levels. The skill's "methods on disjoint subsets of fields" heuristic is the manual-review version of LCOM.

## Cognitive Complexity (Sonar, vendor source)

SonarSource (commercial vendor) publishes Cognitive Complexity as an alternative to McCabe's Cyclomatic Complexity. Per the SonarSource white paper at <https://www.sonarsource.com/resources/cognitive-complexity/>: "a Sonar exclusive metric formulated to more accurately measure the relative understandability of methods." Cognitive Complexity adjusts for nesting depth and for control-flow constructs that are easier or harder for humans to follow than the count-edges-and-nodes formulation.

This is cited here as a vendor-published metric. It is NOT a McCabe replacement endorsed by an open standards body. If the project's static-analysis stack already reports it, take its readings as additional input on top of cyclomatic complexity. If it does not, McCabe's < 10 is the safer foundational reference.

## References

- McCabe (1976), "A Complexity Measure", IEEE Transactions on Software Engineering SE-2(4) pp 308-320.
- Stevens, Myers, Constantine (1974), "Structured design", IBM Systems Journal 13(2) pp 115-139.
- SonarSource white paper, "Cognitive Complexity": https://www.sonarsource.com/resources/cognitive-complexity/ (vendor source).
