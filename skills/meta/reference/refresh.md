# /meta refresh

Check reference skills against the current language or framework release and update their guidance, keeping a capped history of prior versions. Arguments: `<skill name(s)>, --all, or blank to pick from the stale list`.

## What this maintains

Reference skills are every `skills/*/SKILL.md` except the procedure skills `audit`, `write`, `deps`, `investigate`, `pr`, `unslop`, `report-format`, and `meta`. Each keeps its sources in `skills/<name>/sources.md`, which nothing links and only this procedure reads. There, per file, a References list, a Claims list naming each claim's source, and under `## SKILL.md` the Version notes this skill owns:

```markdown
### Version notes

Checked: <YYYY-MM-DD> against <source: library id or URL>

- <version> (<release date>): <what changed for the guidance in this skill>
- <older version>: ...
```

Retention: at most 3 version entries, newest first. A pattern that changed moves into a `### Legacy (<version>)` block at the end of the reference file it came from, one short paragraph, at most 2 legacy blocks per file. Anything older is deleted with a one-line note "Older guidance removed <date>; see git history." Git holds the rest.

## Procedure

1. Scope. The arguments are: $ARGUMENTS
   - Skill names: refresh those.
   - `--all`: every reference skill.
   - Blank: list every reference skill with its `Checked:` date (or "never"), oldest first, and ask which to refresh via AskUserQuestion (multi-select, at most 5 unless the user picks `--all`).
2. Read each selected skill's SKILL.md, every file under its `reference/`, and its `sources.md`. Extract: the claimed current version (intro lines and Version notes), every version-specific claim ("since 3.4", "renamed in 16", "deprecated"), and the `sources.md` URLs.
3. Research, never from memory. For each skill dispatch one researcher agent (Agent tool, subagent_type: kit:researcher, same message for up to 3 at a time) with: the library or framework name, the `sources.md` URLs, the claimed current version, and the list of version-specific claims. Ask it to return, with a source for each:
   - Latest stable version and its release date; latest LTS if the ecosystem has one.
   - Each claim: verified / contradicted (with the current wording) / could not verify.
   - New deprecations or renames since the claimed version.
   - New official best-practice guidance that changes what this skill recommends. Official docs and release notes only; blog posts do not count.

   slop-patterns has no release, so its brief differs. Ask for the Wikipedia page's current revision and its era word lists, papers since the `Checked:` date that measure LLM word or construction frequency, and vendor or community lists as weaker evidence. For each lexicon word or rule, the answer is confirmed, contradicted, or no longer cited.

   Then probe before adding. A candidate word or rule goes into `kit.yml` `slop:` only if it stays quiet on human text. Run `kit slop` with the candidate as a tier 2 rule in a `CLAUDE_CONFIG_DIR` overlay against pre-2022 human prose: GOROOT `src` comments and a repo checked out at a commit before 2022-11. Record its hit count in the Version notes entry. A word with more than a handful of human hits stays out, whatever its source.

4. Classify each skill:
   - `current`: version matches, no contradicted claims. Update only the `Checked:` line.
   - `version bump`: newer release, no guidance change. Add a Version notes entry; update the intro's default-version line.
   - `guidance changed`: at least one contradicted claim or new deprecation. Apply step 5.
   - `could not verify`: researcher had no source. Change nothing; report it.
5. Apply guidance changes, one reference file at a time:
   - Replace the outdated pattern with the current one where it lives.
   - Move the old pattern to `### Legacy (<version>)` at the end of that file, one paragraph stating what changed and when it still applies (projects pinned to the old major).
   - Enforce the caps from "What this maintains". Prune oldest first.
   - Add the Version notes entry naming the source URL or library id from step 3. A changed claim's source goes in its file's Claims list in `sources.md`, never inline.
   - Do not touch guidance the research did not contradict. Do not delete a reference file. Do not reword for style.
6. Report. Write the path `kit scratch-dir refresh` prints, in the report-format skill's format (load it first):
   - Summary: skills checked, per-classification counts.
   - One finding per `guidance changed` skill: severity `warning`, what changed, files edited, source.
   - One finding per `version bump`: severity `info`.
   - "Cannot be verified statically": every `could not verify` item with what a manual check needs.

## Rules

- A claim the researcher could not source stays as it was. "Could not verify" is a valid outcome; a confident edit from training memory is not (`skills/kit-rules/reference/evidence.md`).
- Every Version notes entry cites its source. An entry without one is a defect.
- Prefer the smallest edit that makes the guidance true. Rewriting a reference file is out of scope; propose it in the report instead.
- engineering-fundamentals is exempt, since it holds no version-dependent content. Refresh it only when named explicitly.
- Stop after the report. Do not commit.

## Stop conditions

- Researcher agent unavailable (no Context7, no network): stop and say so.
- If a skill has no `sources.md` URLs and no library the researcher can resolve, report it as unverifiable and move on.
- More than 5 skills selected without `--all`: ask before continuing.
