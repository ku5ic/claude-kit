# Rollback

- A rollback or roll-forward plan exists before launch, written down where everyone involved can reach it, and tested before production.
- Fixing forward is allowed only where a written policy says when.
- In the repo, look for a path that redeploys the previous artifact, a traffic switch (blue-green, canary), or a flag that turns the feature off.
- A new feature can be turned off independently and at once if it misbehaves. For a flag, ask who can flip it without a deploy, what it defaults to when the flag service is down, and when it gets removed.
- Automatic rollback on a bad canary is a question, not a requirement. Its absence is medium at most.
- Data written by the new release has to be readable by the previous one, or rolling back breaks. Ask that question of every schema or format change. The expand-and-contract mechanics are `/code-migrate`'s batching reference.
