# Sessions and their records

## Where they are

- A session's transcript is `projects/<project>/<session>.jsonl` under the Claude config dir (`$CLAUDE_CONFIG_DIR`, else `~/.claude`). `<project>` is the working directory with each `/` turned into `-`.
- Its subagents' transcripts sit in `projects/<project>/<session>/subagents/`, in the same format.
- Transcripts and subagent transcripts are removed after `cleanupPeriodDays`, 30 days by default. Older sessions may be gone, and the report says so.
- The kit's own logs are in `logs/` under the same config dir. `guards.jsonl` has a line per rule fire with its `rule`, `hook`, `event`, `session_id`, and `ts`. `skills.jsonl` has a line per skill activation with its `skill_file` and `session_id`. `claims.jsonl`, when present, records each flagged claim kept or dropped. Filter each by `session_id`.
- `kit stats --since <days>` sums the rule fires and claim outcomes across sessions, for comparing this one with the usual.

## Windows

- A project's sessions are the `*.jsonl` files directly in its `projects/<project>/` folder, dated by their first record that has a `timestamp`. A file with no `user` record (a title or a cost line alone) is no session. Every project is every such folder, and subagent transcripts don't count as sessions.
- Rank the window's sessions from the kit's logs first, cheapest to read: guard blocks per `session_id` in `guards.jsonl`, claims dropped in `claims.jsonl`, and the transcript's size and its `turn_duration` total. Each rank shows its numbers.
- Only the sessions the user picks get read in depth. The rest feed the Patterns counts from the logs alone, and the report says so.
- A window past the cleanup period reaches only what's still on disk. The report names the oldest session found.

## Similar

- A signature is what a confirmed finding has that another session could show the same way: a guard rule's name, a tool error's text with ids and paths stripped, a command that failed, or a hook line.
- Find candidates by counting matches per file (`grep -c` or `rg -c`), never by printing them, then take the files with hits, newest first.
- A candidate is a match only after an analyst reads the matching lines and confirms the same failure, not just the same words.

## Record meanings

- Count the record types first, never assume them: `jq -r '[.type, (.subtype // "")] | join(" ")' <file> | sort | uniq -c`.
- A prompt the user typed is a `user` record whose `message.content` is a string. A `user` record holding a `tool_result` is a tool's output, not the user.
- A `system` record with subtype `compact_boundary` marks a compaction. One with `turn_duration` holds a turn's time.
- Each record has a `timestamp`. A line's number is its `path:line`, from `jq`'s `input_line_number` or `grep -n`.
- Record what each type means in the case file, so the analysts don't rediscover it.

## Context safety

- One line can be megabytes. Measure first: `wc -lc <file>`, and `awk 'length($0) > 100000 {print NR, length($0)}' <file>` for long lines.
- Never `cat` a transcript, or `grep` it for content. Get line numbers and counts first, then small fields from named lines (`sed -n <N>p <file> | jq -c '{...}'`, or `cut -c1-500`).
- Over 500 characters from one record, narrow the field or the slice.
