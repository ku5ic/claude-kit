# Content and grammar tells

Each tell below has a rule in the lexicon; the rule ids are in code spans. The fix is always a sentence that names something.

## Content

| Tell                 | Rule                 | What it does                                                                      | Instead                                                      |
| -------------------- | -------------------- | --------------------------------------------------------------------------------- | ------------------------------------------------------------ |
| Importance claims    | `significance`       | Says a thing matters ("plays a key role", "a testament to") without saying how    | The effect, measured or named                                |
| Empty "-ing" tails   | `ing-tail`           | Bolts an interpretation onto a fact (", highlighting ...", ", ensuring ...")      | The consequence as its own clause with a subject, or nothing |
| Vague connection     | `vague-connection`   | "Was associated with" where the relation is known                                 | The relation: wrote, owns, calls, broke                      |
| Formulaic challenges | `despite-challenges` | "Despite its strengths, it faces challenges" closing sections                     | The specific problem, or no section                          |
| Hedged importance    | `worth-noting`       | "It's worth noting that" before the point                                         | The point                                                    |
| Corrective framing   | `corrective-framing` | Recent-model habit: "should not be confused with", "does not by itself establish" | Say what it is                                               |

## Grammar

| Tell                                                 | Rule                   | Instead                                     |
| ---------------------------------------------------- | ---------------------- | ------------------------------------------- |
| Copula avoidance: "serves as", "stands as", "boasts" | `copula`, `boasts`     | "is", "has"                                 |
| Negative parallelism: "not only X but also Y"        | `not-only`             | Both facts plainly, or the one that matters |
| "It's not X, it's Y"                                 | `not-x-but-y`          | What it is                                  |
| "Y rather than X" when nobody proposed X             | `rather-than` (tier 3) | Y                                           |
| Rule of three: a triad that pads a thin point        | none, judgment         | The one item that matters                   |
| Elegant variation: a new synonym each mention        | none, judgment         | The same name every time                    |

## Vocabulary

Era words (`slop.density.words`) count only as a cluster. One "robust" is fine; four era words in two sentences is the model's voice. Rewrite the passage, don't delete the words one by one: [principles.md](principles.md) says why.

## Formatting

| Tell                                                                           | Rule                                                           | Instead                                                    |
| ------------------------------------------------------------------------------ | -------------------------------------------------------------- | ---------------------------------------------------------- |
| Bold scattered inside prose                                                    | `bold-in-prose` (tier 3)                                       | No bold, or bold on the one term a scanner needs           |
| Bullets that open with a bold label and a colon, in a short text               | none, judgment                                                 | Plain sentences; that shape suits a scanned reference file |
| Title Case Headings                                                            | none                                                           | Sentence case                                              |
| Emoji leading headings or list items                                           | none                                                           | Words                                                      |
| Markdown leaking into a commit, a YAML value, or a chat that doesn't render it | none                                                           | The target format's own markup                             |
| The em dash (U+2014) cadence: punched-up clauses                               | sanitizer turns the character into a hyphen; the cadence stays | A comma or two sentences                                   |
| A joining colon: a clause, a colon, then the reveal or a restatement           | none, judgment ([false-positives.md](false-positives.md))      | Two sentences, or a comma                                  |

The joining colon is the em dash's cadence in other punctuation, so swapping one for the other fixes nothing. Colons in code, paths, key-value pairs, times, ratios, and a list's introduction are fine.

## Sources

- Wikipedia, "Signs of AI writing": its Content, Language and grammar, and Style sections cover every rule above. The em dash section says models use the dash where people use commas, parentheses, or colons.
- Reinhart et al., arXiv 2410.16107: present participial clauses at 5.3x the human rate in GPT-4o (`ing-tail`).
- Pangram, "Comprehensive guide to spotting AI writing patterns": "serves as", "is a testament to", "not only ... but also", "it's important to note".
- blader/humanizer: negative parallelism, vague association, bold as decoration.
