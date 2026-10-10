# /meta skill

Draft a pattern skill pack for a stack from current docs and this repo's precedent, plus its `kit.yml` wiring. Arguments: `<stack name, e.g. graphql or opentofu>`.

## Procedure

1. Scope. The stack is: $ARGUMENTS. Blank: ask via AskUserQuestion. The pack name is `<stack>-patterns`, lowercase and hyphenated. If `skills/<name>/` already exists in the kit, stop and point at `/meta refresh <name>` instead.
2. Research. Dispatch the researcher agent (subagent_type: kit:researcher, foreground) with fully resolved text, never a bare link. Ask it for, through Context7 first and official docs second:
   - the library id and current stable version, with its release date
   - the key APIs and idioms a reviewer would check, and what each replaced
   - deprecations and removals in the last two majors
   - the review-time mistakes the docs themselves warn about

   One source per claim. A claim it returns without a source doesn't go in the pack.

3. Precedent. If `<repo-context>` or `kit detect-stack` shows the current repo uses the stack, load the `investigate` skill and answer "how does this repo use <stack>: file layout, conventions, config" with `file:line` citations. A pack rule that contradicts the repo's own consistent usage gets named in the draft, not silently dropped.
4. Draft. Work in the kit's source checkout, the directory `$CLAUDE_KIT_DEV` names, never an installed plugin copy or the marketplace clone. Unset: stop and ask where the checkout is. Copy `templates/pattern-skill/` to `skills/<name>/` and fill it:
   - `description`: the file and dependency signals, then the concepts, in the template's shape.
   - One `reference/<area>.md` per area with enough verified material; three to five areas is typical. Every rule cites its source URL or library id.
   - `reference/anti-patterns.md`: the mistakes from step 2, each with a severity from the rubric.
   - `## Version notes`: one entry for the current version, with today's date and the step 2 source, in the format `/meta refresh` maintains.
   - Leave no `<placeholder>` behind. `rg -n '<[a-z]' skills/<name>/` prints nothing when done.
5. Wire it in `kit.yml`, reading two existing entries of each key first and matching their shape:
   - Detection: a `dependency_skills` entry whose `deps:` are the packages that signal the stack, as its manifests declare them, and whose `skills:` lists the pack.
   - `skill_file_map`: the file globs that should trigger the pack, if the stack has its own file types.
   - `skill_triggers`: one phrase naming a concrete action ("before writing ..."), like the existing ones.
   - When the stack has its own task runner: a reader in `go/internal/config/grammar.go`. Its check commands or formatter: a `toolchain_checks` or `formatters` entry. Anything that runs a binary resolves it locally, never through `npx` or another installer.
6. Verify. Run `go test ./...` in `"$CLAUDE_KIT_DEV/go"`; it checks the new skill against `kit.yml`. Fix any failure.
7. Report the path of the new skill directory, the `kit.yml` keys touched, and the one manual step: add `"Skill(<name>)"` to `permissions.allow` in the user's `settings.json`.

## Rules

- Nothing from memory. A version, API shape, or deprecation not traced to step 2's sources stays out of the pack.
- Cut anything no repo using this stack would hit; a pack is a review aid, not a tutorial.
- Don't edit `settings.json`; it's the user's config.
- Pin `model:` or `effort:` only when it differs from the session default. Never both on one skill: Claude Code drops the model override when both are present. A skill forks (`context: fork`) only to carry a `model:` pin or to name an `agent:`.
