# 13. Generate the chat bundle from the same files

Date: 2026-11-11

## Status

Accepted

## Context

claude.ai doesn't install a plugin with a top-level `bin/`, so the rules and writing skills only reached Claude Code (PR #13). A second copy for chat would drift from the first ([8](0008-give-each-topic-one-owner.md)).

## Decision

`kit bundle chat` builds a smaller plugin zip from the same files, chosen in `kit.yml` `chat_bundle:`. A `<!-- chat: skip -->` block is dropped from the chat copy, and a `<!-- chat: only` block appears only there, so one source file reads right in both places. `--check` fails CI when a bundled file gains a Claude Code dependency. The release attaches the zip. Made in 0.8.0 (PR #13).

## Consequences

- A bundled file can't name a `kit` command, `kit.yml`, a `skills/` path, or AskUserQuestion outside a skip block. `go/internal/chatbundle/lint.go` holds the list.
- In chat, no guard runs, so a skill that relies on a guard in Claude Code needs its chat-only line.
- Rule citations are rewritten to the chat skill's names when the bundle is built.
