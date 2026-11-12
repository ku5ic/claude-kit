# slop-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

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
- Taking Responsibility for AI-Generated Content, the company page behind the reader-cost checks in `reference/principles.md`.

### Version notes

Checked: 2026-11-10 against Wikipedia revision 1384793302 and the papers above

- Wikipedia rev 1384793302 (2026-11-10): four era word lists, the newest "mid-2026 and on" citing one product review. Added `realm`, `groundbreaking`, `commendable`, `enduring`, `deep dive`, and the missing inflections; dropped `prioritize`, `practical`, `clearer`, `steady`, `seamless`. Human-corpus tier 2 hits went from 13 to 12.

## reference/principles.md

### Claims

- Density, relevance, and tone predict a slop label better than word choice: "Measuring AI 'Slop' in Text", arXiv 2509.19163.
- Wikipedia's mid-2026 word list cites a single product review, so the words that rest on it alone (`quietly`, `dependable`, `echoed`, `universally`) are the weakest on the list. The page is CC BY-SA, summarized in the kit's own words, never quoted.
- The human-text test corpus is pre-2022 Go stdlib comments and Markdown docs, 151k words. `prioritize`, `practical`, `clearer`, `steady`, and `seamless` failed it: they're common in human technical prose, or had no source.
