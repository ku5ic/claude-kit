# Principles

## Tells and slop

A tell is evidence a model wrote the text. Slop is a verdict on the text: the reader paid more attention than it returned. A text can carry tells and still be useful, or be free of tells and still be empty.

When people label text as slop, three things predict the label far better than word choice: how dense it is, whether it's relevant to the reader's purpose, and its tone. Vocabulary is a small part of tone. (Measuring AI "Slop" in Text, arXiv 2509.19163.)

Working definition here: slop is effort moved from the writer to the reader.

## Where tells come from

A model drifts toward the statistical average, and preference tuning pushes it toward prose nobody objects to. The specific, rare fact gets swapped for a generic claim that sounds important. The result is less specific and more inflated at the same time.

That one pattern, generic plus inflated, sits under most of the tells in this skill. Hunt it, not the words.

## Why the word lists rotate

Each model generation has its own overused words; a word that marks 2024 output is ordinary by 2026, and people pick up model vocabulary too. So the lexicon treats single era words as noise and only counts a cluster (tier 3, warn only). Structural patterns, the empty "-ing" clause, the importance claim, the copula swap, last across generations; they're tier 2.

## Tiers

| Tier | What                                                                     | What happens                                                |
| ---- | ------------------------------------------------------------------------ | ----------------------------------------------------------- |
| 1    | Unambiguous: chat filler in a file, leaked citation markup, placeholders | Blocks                                                      |
| 2    | Structural constructions                                                 | Blocks                                                      |
| 3    | Era vocabulary and soft markers, counted per 100 prose words             | A note when the density crosses the threshold; never blocks |

## Sources

- Wikipedia, "Signs of AI writing" (CC BY-SA; summarized here in my own words, not quoted). Its mid-2026 word list cites a single product review, so the words that rest on it alone (`quietly`, `dependable`, `echoed`, `universally`) are the weakest on the list.
- "Measuring AI 'Slop' in Text", arXiv 2509.19163
- Kobak et al., excess vocabulary in PubMed abstracts, arXiv 2406.07016
- Juzek and Ward, "Why does ChatGPT delve so much?", arXiv 2412.11385
- Liang et al., LLM-modified text in arXiv abstracts (2404.01268) and in peer reviews (2403.07183)
- Reinhart et al., "Do LLMs write like humans?", arXiv 2410.16107
- "The Last Fingerprint", arXiv 2603.27006
- Pangram, "Comprehensive guide to spotting AI writing patterns", a detector vendor and weaker evidence
- blader/humanizer, a community list and weaker evidence

A word enters the list only when a source backs it and it stays quiet on human text. Pre-2022 Go stdlib comments and Markdown docs are the test, at 151k words. `prioritize`, `practical`, `clearer`, `steady`, and `seamless` failed: they're common in human technical prose, or had no source.
