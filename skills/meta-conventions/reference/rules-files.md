# Rules files

- A rule without `paths` loads at session start. A rule with `paths` loads when Claude reads, writes, or edits a matching file.
- Keep each instruction file under 200 lines; a longer one costs context and lowers adherence.
- Write each line as something checkable ("handlers live in `src/api/handlers/`"), never "keep files organized".
- Two conflicting instructions get resolved arbitrarily. Before writing a line, check the project's CLAUDE.md files and other rules for one it contradicts.
