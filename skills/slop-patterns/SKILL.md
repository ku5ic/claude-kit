---
name: slop-patterns
description: How to rewrite text a slop guard flagged, and how to keep AI tells and empty filler out of prose, commit messages, PR text, code comments, docs, chat replies, and UI. Load when a guard flags AI tells, or before drafting a commit message, PR text, a README, docs, or a landing page.
user-invocable: false
---

# Slop patterns

Two layers, kept apart:

- **AI tells**: surface patterns that show up far more in model output than in human writing.
- **Slop**: text that costs the reader more than it gives. No regexp sees it. Density, relevance, and specificity decide it.

<!-- chat: skip -->

`kit slop` and the guards find AI tells with the lexicon in `kit.yml` `slop:`.

<!-- chat: end -->

<!-- chat: only
In claude.ai chat, no guard runs. The word lists and tiers are in `reference/lexicon.md` in the unslop skill.
-->

Fixing the first without the second makes the text harder to spot and no better to read. A flagged sentence gets rewritten around a fact, never patched with a synonym: [reference/rewrite.md](reference/rewrite.md).

Loading this skill isn't enough. Before your next edit, read the file of every row that matches it. The rules live in those files, and this one only points at them.

| Before                                               | Read                                                                                             |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| Writing any prose for the first time this session    | [reference/principles.md](reference/principles.md), [reference/rewrite.md](reference/rewrite.md) |
| Rewriting prose a scan or guard flagged              | [reference/content-and-grammar.md](reference/content-and-grammar.md)                             |
| Writing a commit message, PR text, or a review reply | [reference/commits-and-prs.md](reference/commits-and-prs.md)                                     |
| Writing a code comment or docstring                  | [reference/code-comments.md](reference/code-comments.md)                                         |
| Writing a README or docs page                        | [reference/docs-and-readme.md](reference/docs-and-readme.md)                                     |
| Writing a chat reply or a subagent report            | [reference/chat.md](reference/chat.md)                                                           |
| Writing UI, its CSS, or copy in it                   | [reference/design.md](reference/design.md)                                                       |
| Acting on a hit that looks wrong                     | [reference/false-positives.md](reference/false-positives.md)                                     |
| Undoing a rewrite because it reads stiff             | [reference/human-signals.md](reference/human-signals.md)                                         |

The word lists, tiers, and thresholds live only in the lexicon; these references explain them and never copy them.

Length and shape belong to `skills/kit-rules/reference/output.md`; comment rules to `skills/kit-rules/reference/change.md` section 5.
