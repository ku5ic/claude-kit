# Commit message practice

## Subject

- 50 characters or fewer. 50 is git's soft limit; 72 is the body's wrap width, not the subject's.
- Imperative mood: "Fix bug", not "Fixed bug" or "Fixes bug".
- No trailing period.
- An `area:` prefix is common where the project uses one.

## Blank line

- A body follows one blank line. git reads everything up to the first blank line as the title, and format-patch makes it the email subject.

## Body

- Wrap at about 72 characters.
- The problem first, in present tense: "The code does X when given Y".
- Then why this change fixes it, and the alternatives dropped, when there were any.
- Self-contained: summarize a discussion instead of linking it.
- Cite another commit as its abbreviated hash, then its subject and date in parentheses.
