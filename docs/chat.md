<img src="assets/claude-kit-mark.svg" alt="" width="40">

# claude-kit in claude.ai chat

Each release attaches `kit-chat-<version>.zip`, a smaller plugin for chat on the web, desktop, and mobile. Upload it under Customize > Plugins, with code execution on. Updates mean uploading the new zip; it replaces the old one.

- **What's in it:** the `kit-rules` skill, built from the output, evidence, and change rules, plus `investigate`, the ten `write-*` commands, `unslop`, `slop-patterns`, and `engineering-fundamentals`. They work on what you paste; chat has no repo. `kit.yml` `chat_bundle:` lists them.
- **No guards.** Chat runs no hooks, so nothing blocks, formats, or checks. `unslop` scans with a bundled `kit`, and reads its lexicon by hand if that can't run.
- **The rules can be skipped.** `kit-rules` loads when the model takes a task for writing or coding, and nothing enforces that. Only your claude.ai preferences reach every chat.
- **Mobile has no `/` menu.** Skills load from what you ask for.
