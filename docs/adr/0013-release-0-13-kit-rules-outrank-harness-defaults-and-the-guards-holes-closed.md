# 13. Release 0.13: kit rules outrank harness defaults, and the guards' holes closed

Date: 2026-11-20

## Status

Accepted

Amends [12. Release 0.12.1: mods for what the kit draws](0012-release-0-12-1-mods-for-what-the-kit-draws.md)

## Context

Sources are the commits on `feat/release-0-13-0-kit-rules-outrank-harness-defaults`, each named where it's used, and the files they change.

- Before `dba9678`, nothing in the system prompt put the kit's rules ahead of a mode's or the harness's own instructions, and before `6ec8945` the kit's hook context carried no label telling it from the harness's.
- Before the fixes below, a credential read passed through a compound command's redirect (`8335f86`), NotebookEdit (`cc0cc54`), an MCP tool's path (`eda6be1`), or a Bash write target (`d00cfb7`), and a report or post written through Bash skipped the claims gate (`cee3aac`).
- Unverified text reached the user and the guards looped: a subagent report confirmed claims and the verifier had no git (`a0ce347`), main-session reports shipped unchecked claims (`57e6854`), 264 of 265 rules-gate denies came from subagents (`4a6c019`), one text was sent back for size eight times (`6e27e2e`), and a test already failing on main blocked every edit turn (`d1d60b5`).
- ADR 9's Config decision says new behavior gets no new block in `kit.yml`. `attribution:` and `architecture_dir:` arrived after it, in `ca78f28`, with no ADR.

## Decision

- A mod, `mods/precedence`, appends a `kit:precedence` system-prompt section: the kit's rules win over default, mode, and built-in skill instructions where they conflict (`dba9678`). This amends ADR 12, which covered only what the kit draws. A system-prompt section sits at the top of a thread, so the mod's `prompt.submit` hook also puts a one-line `kit:` reminder on every prompt, as context the user never sees (`db39bb3`). Both have the `precedence` switch, and the reply-length and step-pause clauses stay in the rules files for when it's off. Every prompt also states its reply ceiling, the chat one included, where it was stated once at session start (`2e2e678`), and the rule that every fact shown the user comes from a tool result this session (`574602e`). Opinion and detail words, and `!long`, lift the ceiling (`d79f6c1`).
- A claim no tool checked is cut, and no label (`likely`, "not run") makes it sayable or clears a hold (`990627b`). A commit body is its reason in 30 words or fewer (`5eb2b31`).
- Every hook context opens with `kit:`, and the nudges state facts (`6ec8945`, `857f7c8`, `30a802f`). Blocks keep their `kit <area>: BLOCKED` headline.
- A second mod adds `/kit-stats`, behind its own `kit_stats` switch (`ac283e9`).
- A plan line `<!-- critic: skip <reason> -->` skips plan-critic, logged and counted, and the plan's claims are still checked (`d71c861`).
- New blocks: the credential routes above, more credential paths (`649f904`), `git -c core.hooksPath`, an AI tool as `--author`, and what `git rebase -x` runs (`055656f`), core.hooksPath through `GIT_CONFIG_KEY_<n>` or `GIT_CONFIG_PARAMETERS`, on the call or exported earlier, and what `git rebase --exe` runs (`24234dd`), a subagent's scratch report a Bash command wrote with unchecked claims (`cee3aac`), and the main thread's code edit, or launch of an agent that could edit, after a `/code-review` until the user answers, by a typed prompt or a question, unless it was a `--fix` review (`b726730`, `f99b56d`).
- A git move's checks are skipped when nothing else in the command writes into the repo, readers piped beside it allowed (`896653d`, `910789a`). Beside other commands, the files that match HEAD after it are left out of bash-edits and the Stop set (`d1d60b5`).
- A Stop gate failing on the branch passes when every failure line also shows in its run on the git base (`d1d60b5`). A one-line reply calling the edits review fixes counts as a review for plan-done (`3e3b239`).
- MCP path rules run on the local file servers `connectors.path_servers` names, and on any MCP path starting with `~`, `/`, or `./`, so a remote server's repo path isn't read as a local file (`fc8cdfe`). A `sed -i` script is no write target (`7f93f59`), and a `cp`, `mv`, or `install` into a directory names each file in it (`910789a`). guard-pr reads a title or body that is only `$(cat <file>)` like `--body-file`, so `/pr-publish`'s title form runs (`8d8b632`).
- New asks: history git can't bring back and the whole project sent to another host (`055656f`), a deleted test file (`6e78b57`), and an edit that changes a task the gates run (`19529aa`).
- The claims gate checks `/deps`, `/pr-review`, and plan-critic reports, terse claims and headings, and what `gh` and `glab` post (`cee3aac`). It holds a text that still states a flagged claim until the checker, run again on the same claim-check id, confirms it (`cc25c5d`, `2c04a2b`), and rechecks a revised subagent report (`bc4c1e3`).
- Every skill's scratch report but `/unslop`'s and a prompt's gets the hook's own run, in the main session too, sent back once per destination (`57e6854`). That replaces `3ce08c4`'s `kit:` note for main-session reports, so ADRs 8 and 9 stand for them: ADR 9 kept the model from choosing when its own claims get checked.
- Before any model run, bare references (`file:line`, a path, a SHA, a backticked symbol) are settled in the repo, and a finding whose Evidence names a read or command the session never ran is flagged (`14f801f`, `57e6854`). The rest goes to Haiku, whose evidence is direct tool results only, with WebFetch and `kit read`, a git and code-host subcommand that refuses writing flags (`a0ce347`, `7c245ad`). A chat reply gets the no-model pass at Stop, corrected on the next turn (`c292b51`).
- ExitPlanMode runs size, citations, the critic, and the claim check on every submit, with one deny naming each reason, and a plan revised after its critique gets the no-model pass (`0383dc0`). A resubmit asks for a list of what changed (`24fe22d`).
- Size sends each text back once per destination (`6e27e2e`). `joining-colon` is a default slop rule, and an edit's slop deny stops after two per file, naming `unslop: allow` (`38fba98`). Blocks log the subagent's type, and asks are logged (`4a6c019`).
- An audit offers its read-only runtime checks after the report and runs the ones the user picks (`7fc4349`). `/audit-tests` finds tests that can't fail, test the implementation, duplicate another, or are trivial (`5e9a5e6`).
- The rules gate denies on a missing skills log, fails open with a notice on one it can't read (`121d396`), and counts a skill read only in a skills dir, and a gate skipped with code left unchecked runs again at Stop (`d3176c4`). A done plan's Stop holds until a review launches, and its second retry goes through with a notice (`cf04879`, `7e03ac1`).
- A session's first fail-open is shown to the user (`5ab281b`).
- `claudecli` owns the failed-run cooldown, and a failed verifier run pauses the claim check for an hour, or only its text when the run hit its budget, turn, or time limit (`a3984a2`, `ba2cee0`). The text goes back to Claude once to check itself (`57e6854`). The verifier is denied every `sensitive_paths` read (`6125b9c`).
- Three holes stay open and are listed in `SECURITY.md`'s known gaps (`216f792`): one plan-critic run covers later versions of the plan, which get only the no-model pass, a background agent's edit after the turn counts as the user's, and Bash writes are checked after they land.
- Of ADR 9's Config decision, this holds: `kit.yml` stays small, and data another file holds is derived from it. A new key needs the author's yes, as `CLAUDE.md` says, which `attribution:`, `architecture_dir:`, `connectors.path_fields` (`eda6be1`), and `connectors.path_servers` (`fc8cdfe`) had.
- Deferred: three readers of a project's declared dependencies, `sources/deps.go`, `codegraph/manifests.go`, and `pkgcheck/imports.go`, stay apart.

## Consequences

- `bash-dispatch` and `bash-edits` run under 300 s timeouts, so a verifier run fits (`cee3aac`). Git calls end a second before the hook's own timeout (`cf04879`).
- During an outage, and for a text whose transcript can't be read, Claude checks its own claims with tools, and the user sees no `NOT CHECKED` notice (`57e6854`).- A changed Rake task body raises no ask, since Rake tasks are read by name only (`go/internal/sources/providers.go`).
