# slop-patterns sources

The sources and version notes behind this skill, for `/meta-refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

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
- Anthropic, Harness design for long-running apps: https://www.anthropic.com/research/harness-design-long-running-apps
- Taking Responsibility for AI-Generated Content, the company page behind the reader-cost checks in `reference/principles.md`.

### Version notes

Checked: 2026-11-26 against Wikipedia revision 1387897756 and the papers above

- Wikipedia rev 1387897756 (2026-11-25): added `causal`, `correlate`, and `challenge`, from the Grok and mid-2026 lists. Probed as tier 2 rules on 2.37M words of Go 1.26 standard-library comments, they hit 0, 2, and 4 times; `valuable` (6), `empirical` (7), `matters` (49), and `key` (1,981) stayed out. No Go tree from before 2022 was on hand, so the corpus is newer than the 151k-word one. `comprehensive`, `realm`, and `commendable` weren't found on either revision's lists, and stay pending a check of where they came from. https://en.wikipedia.org/w/index.php?title=Wikipedia:Signs_of_AI_writing&action=raw&oldid=1387897756
- Wikipedia rev 1384793302 (2026-11-10): four era word lists, the newest "mid-2026 and on" citing one product review. Added `realm`, `groundbreaking`, `commendable`, `enduring`, `deep dive`, and the missing inflections; dropped `prioritize`, `practical`, `clearer`, `steady`, `seamless`. Human-corpus tier 2 hits went from 13 to 12.

## reference/principles.md

### Claims

- Density, relevance, and tone predict a slop label better than word choice: "Measuring AI 'Slop' in Text", arXiv 2509.19163.
- Wikipedia's mid-2026 word list cites a single product review, so the words that rest on it alone (`quietly`, `dependable`, `echoed`, `universally`) are the weakest on the list. The page is CC BY-SA, summarized in the kit's own words, never quoted.
- The human-text test corpus is pre-2022 Go stdlib comments and Markdown docs, 151k words. `prioritize`, `practical`, `clearer`, `steady`, and `seamless` failed it: they're common in human technical prose, or had no source.

## reference/design.md

### Claims

- The four criteria in section 12 and their order: "Harness design for long-running apps", published 2026-03-24, checked 2026-11-25. Its author weighted design quality and originality over craft and functionality, where Claude "already scored well".
