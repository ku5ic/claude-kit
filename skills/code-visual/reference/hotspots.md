# Hotspot inputs and their limits

- scc's complexity counts branch and loop keywords per file, with no syntax tree, so it compares files of one language only. Rank within a language, or say in the report that a cross-language ranking mixes scales.
- `git log --follow` takes a single file, so a path-wide churn count loses the commits before a rename. Say so when a hotspot's folder was renamed in the window.
- Rename detection defaults to 50% similarity; `-M90%` tightens it, `--no-renames` turns it off.
