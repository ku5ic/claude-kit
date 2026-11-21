# False positives

The scan already skips fenced code, code spans, double-quoted spans, blockquotes, frontmatter, and string literals in code. Most "hits" on correct text disappear once the example is quoted or put in backticks.

## Words that are right in context

| Word              | Correct when                                                                                               |
| ----------------- | ---------------------------------------------------------------------------------------------------------- |
| `robust`          | Statistics: "a robust estimator"                                                                           |
| `leverage`        | Finance, or a lever                                                                                        |
| `underscore`      | The `_` character                                                                                          |
| `landscape`       | Page orientation                                                                                           |
| `foster`          | "A foster parent"                                                                                          |
| `elevated`        | A design system's elevation variant                                                                        |
| `associated with` | A data relation: "the token associated with the session" (the rule only fires on "was/is associated with") |

## The joining colon's rule blocks some human text

People write it too. A regex for three lowercase words, a colon, then two more (`\b[a-z]+ [a-z]+ [a-z]+: [a-z]+ [a-z]+`) skips labels, commit prefixes, times, and URLs. It hit 0.12 times per 1,000 words in pre-2022 Go stdlib comments ("lost the race: try again") and 1.7 in model-written docs. A quarter of its hits were list introductions. No study names the construct.

`joining-colon` blocks it at tier 2 anyway, by the author's choice. Its match needs the sentence to end after the colon with no comma, semicolon, code span, or parenthesis, so a list's introduction ("in this order: a, b, c") passes. A human sentence it still blocks takes an allow marker.

<!-- chat: skip -->

## Quoting tells on purpose

Docs about tells (this skill included) quote them. Put each example in quotes or a code span. When that doesn't fit, put a marker on the line above:

```markdown
<!-- unslop: allow quoting the research table -->
```

```go
// unslop: allow the upstream error text, verbatim
```

The marker exempts the next non-blank line. The reason is required and every use is logged, so it's a reviewed exception, not a way past the guard.

## Whole files

Generated or third-party files go in `slop.skip` (fixtures, snapshots, testdata are there already). A personal addition goes in your overlay, `~/.claude/claude-kit.local.yml`. Turning one rule off: `disabled_rules: [slop-<id>]`.

<!-- chat: end -->
