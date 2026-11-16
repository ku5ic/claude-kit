# Ranking

- One business day is the most a review request should wait, so the age since the request leads among review requests.
- A slow review slows the whole team, not only the author.
- Small changes get reviewed faster and fit in a few minutes. Among equals, the smaller PR goes first. Around 1000 changed lines is usually too large, and the row says it needs a block of time.
- Blocked on the user outranks age: the user's approval is the last one missing, or the author answered the user's last review.
- The user's own PRs go red CI first, then unanswered review comments, then approved and mergeable.
- Drafts come last and are marked, never ranked as review work.
- Each row shows blocked-on-me, age, and size, so the order can be checked. No source gives a queue order, so the report calls this ranking a heuristic.
