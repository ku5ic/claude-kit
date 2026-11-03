# Principles

## Tells and slop

A tell is evidence a model wrote the text. Slop is a verdict on the text: the reader paid more attention than it returned. A text can carry tells and still be useful, or be free of tells and still be empty.

When people label text as slop, three things predict the label far better than word choice: how dense it is, whether it's relevant to the reader's purpose, and its tone. Vocabulary is a small part of tone. (Measuring AI "Slop" in Text, arXiv 2509.19163.)

Working definition here: slop is effort moved from the writer to the reader.

## Where tells come from

A model drifts toward the statistical average, and preference tuning pushes it toward prose nobody objects to. The specific, rare fact gets swapped for a generic claim that sounds important. The result is less specific and more inflated at the same time.

That one pattern, generic plus inflated, sits under most of the tells in this skill. Hunt it, not the words.

## Why the word lists rotate

Each model generation has its own overused words; a word that marks 2024 output is ordinary by 2026, and people pick up model vocabulary too. So `kit.yml` treats single era words as noise and only counts a cluster (tier 3, warn only). Structural patterns, the empty "-ing" clause, the importance claim, the copula swap, last across generations; they're tier 2.

## Tiers

| Tier | What                                                                     | What happens                                                |
| ---- | ------------------------------------------------------------------------ | ----------------------------------------------------------- |
| 1    | Unambiguous: chat filler in a file, leaked citation markup, placeholders | Blocks                                                      |
| 2    | Structural constructions                                                 | Blocks                                                      |
| 3    | Era vocabulary and soft markers, counted per 100 prose words             | A note when the density crosses the threshold; never blocks |

## Sources

- Wikipedia, "Signs of AI writing" (CC BY-SA; summarized here in my own words, not quoted)
- "Measuring AI 'Slop' in Text", arXiv 2509.19163
- Kobak et al., excess vocabulary in PubMed abstracts, arXiv 2406.07016
- Juzek and Ward, why ChatGPT overuses "delve" (FSU)
- "The Last Fingerprint", arXiv 2603.27006
