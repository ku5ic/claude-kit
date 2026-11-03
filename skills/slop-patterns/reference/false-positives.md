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
