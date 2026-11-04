# Chat replies and subagent reports

Length and shape are `rules/output.md` section 1; this covers the phrases.

<!-- chat: skip -->

A chat reply can't be blocked: Claude Code shows it before any hook can see it. The Stop hook scans it and the next prompt carries a correction. A subagent's report is scanned before the parent reads it and sent back once.

<!-- chat: end -->

For the commit shape see `skills/write/reference/commit.md`, and `rules/change.md` sections 5, 8, and 10.

Honest uncertainty isn't a tell. "Not run", "not sure, likely X" stay: `rules/evidence.md` section 1.
