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

| Writing                                   | Read                                                                                             |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------ |
| Anything, first time                      | [reference/principles.md](reference/principles.md), [reference/rewrite.md](reference/rewrite.md) |
| Prose a scan or guard flagged             | [reference/content-and-grammar.md](reference/content-and-grammar.md)                             |
| A commit message, PR text, a review reply | [reference/commits-and-prs.md](reference/commits-and-prs.md)                                     |
| Code comments and docstrings              | [reference/code-comments.md](reference/code-comments.md)                                         |
| A README or docs page                     | [reference/docs-and-readme.md](reference/docs-and-readme.md)                                     |
| A chat reply or a subagent report         | [reference/chat.md](reference/chat.md)                                                           |
| UI: components, CSS, copy in the UI       | [reference/design.md](reference/design.md)                                                       |
| A hit that looks wrong                    | [reference/false-positives.md](reference/false-positives.md)                                     |
| Second-guessing a rewrite as too stiff    | [reference/human-signals.md](reference/human-signals.md)                                         |

The word lists, tiers, and thresholds live only in the lexicon; these references explain them and never copy them.

Length and shape belong to `skills/kit-rules/reference/output.md`; comment rules to `skills/kit-rules/reference/change.md` section 5.

<!-- chat: skip -->

## References

- Wikipedia, Signs of AI writing: https://en.wikipedia.org/wiki/Wikipedia:Signs_of_AI_writing
- Kobak et al., excess vocabulary in PubMed: https://arxiv.org/abs/2406.07016
- Juzek and Ward, why ChatGPT delves: https://arxiv.org/abs/2412.11385
- Liang et al., LLM-modified arXiv abstracts: https://arxiv.org/abs/2404.01268
- Liang et al., LLM-modified peer reviews: https://arxiv.org/abs/2403.07183
- Reinhart et al., grammatical features of LLM text: https://arxiv.org/abs/2410.16107
- The Last Fingerprint, em dash rates by model: https://arxiv.org/abs/2603.27006
- Measuring AI slop: https://arxiv.org/abs/2509.19163
- Pangram, AI writing patterns (vendor, weaker evidence): https://www.pangram.com/blog/comprehensive-guide-to-spotting-ai-writing-patterns
- blader/humanizer (community, weaker evidence): https://github.com/blader/humanizer
- Anthropic frontend-design skill: https://github.com/anthropics/skills/tree/main/skills/frontend-design

## Version notes

Checked: 2026-11-10 against Wikipedia revision 1384793302 and the papers above

- Wikipedia rev 1384793302 (2026-11-10): four era word lists, the newest "mid-2026 and on" citing one product review. Added `realm`, `groundbreaking`, `commendable`, `enduring`, `deep dive`, and the missing inflections; dropped `prioritize`, `practical`, `clearer`, `steady`, `seamless`. Human-corpus tier 2 hits went from 13 to 12.

<!-- chat: end -->
