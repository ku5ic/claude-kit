---
name: slop-patterns
description: How to rewrite text a slop guard flagged, and how to keep AI tells and empty filler out of prose, commit messages, PR text, code comments, docs, chat replies, and UI. Load when a guard denies a write or a commit for AI tells, or before writing a README, docs, or a landing page.
user-invocable: false
---

# Slop patterns

Two layers, kept apart:

- **AI tells**: surface patterns that show up far more in model output than in human writing.
- **Slop**: text that costs the reader more than it gives. No regexp sees it. Density, relevance, and specificity decide it.

<!-- chat: skip -->

`kit slop` and the guards find AI tells with the lexicon in `kit.yml` `slop:`.

<!-- chat: end -->

Fixing the first without the second makes the text harder to spot and no better to read. A flagged sentence gets rewritten around a fact, never patched with a synonym: [reference/rewrite.md](reference/rewrite.md).

| Writing                                   | Read                                                                                             |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------ |
| Anything, first time                      | [reference/principles.md](reference/principles.md), [reference/rewrite.md](reference/rewrite.md) |
| Prose a guard flagged                     | [reference/content-and-grammar.md](reference/content-and-grammar.md)                             |
| A commit message, PR text, a review reply | [reference/commits-and-prs.md](reference/commits-and-prs.md)                                     |
| Code comments and docstrings              | [reference/code-comments.md](reference/code-comments.md)                                         |
| A README or docs page                     | [reference/docs-and-readme.md](reference/docs-and-readme.md)                                     |
| A chat reply or a subagent report         | [reference/chat.md](reference/chat.md)                                                           |
| UI: components, CSS, copy in the UI       | [reference/design.md](reference/design.md)                                                       |
| A hit that looks wrong                    | [reference/false-positives.md](reference/false-positives.md)                                     |
| Second-guessing a rewrite as too stiff    | [reference/human-signals.md](reference/human-signals.md)                                         |

<!-- chat: skip -->

The word lists, tiers, and thresholds live only in `kit.yml` `slop:`; these references explain them and never copy them.

<!-- chat: end -->

Length and shape belong to `rules/output.md`; comment rules to `rules/change.md` section 5.
