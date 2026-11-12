---
name: <stack>-patterns
description: <Stack> patterns for <three to five concrete areas>. Load before any edit to <file signals>, however small, or for <stack> questions.
---

# <Stack> patterns

Default assumption: <Stack> <current major.minor>.

- <Which rules carry over to the previous major, and where the deltas are called out.>
- Adapt advice to the version in the project's manifest or lockfile.

## Reference files

Loading this skill isn't enough. Before your next edit, read the file of every row that matches it. The rules live in those files, and this one only points at them.

| Before                                       | Read                                                     |
| -------------------------------------------- | -------------------------------------------------------- |
| <an action this file decides, in one line>   | [reference/<area>.md](reference/<area>.md)               |
| Writing, changing, or reviewing <Stack> code | [reference/anti-patterns.md](reference/anti-patterns.md) |
