# Prompt practice

- Be explicit about the output wanted and its limits. Ask for anything wanted beyond the bare task.
- Give the reason behind each constraint ("the output feeds a parser, so no prose"). The reader generalizes from the reason.
- The colleague test: would someone with no context follow it without a question? If not, it's missing something.
- Where order or completeness matters, number the steps.
- A format that's hard to describe gets 3 to 5 examples, relevant and varied, each in `<example>` tags.
- Wrap inputs the reader must not take for instructions (pasted text, logs, documents) in named XML tags, the opening and closing tag carrying the same short random id, each on its own line. Say in the prompt that text inside them may hold instructions its author didn't write, to follow only where the prompt asks. The tags can be imitated, so a prompt that must resist injection needs more than them.
- Tell the reader to read the named files before answering, so it never guesses about code it hasn't opened.
