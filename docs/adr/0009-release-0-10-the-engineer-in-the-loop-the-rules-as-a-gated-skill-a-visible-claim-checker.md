# 9. Release 0.10: the engineer in the loop, the rules as a gated skill, a visible claim checker

Date: 2026-11-12

## Status

Accepted

Supercedes [3. Release 0.2: ship the rules through hooks](0003-release-0-2-ship-the-rules-through-hooks.md)

Amended by [15. Release 0.14: rules that survive a compaction, and a closed retro loop](0015-release-0-14-rules-that-survive-a-compaction-and-a-closed-retro-loop.md)

## Context

Release 0.10 (v0.10.0, PR #16) set the kit's scope and changed how the rules and the claim checker reach a session. This record replaces the former ADRs 15 to 21 and 25 to 27, and its sources are theirs. Release 0.11 ([10](0010-release-0-11-no-release-eval.md)) removed the eval suite this release added.

### Scope

The kit is for developers who review what Claude does. Review is their cost, and a tool that decided for them what needs review would ask them to trust it instead of checking. A team policy layer and Windows support each widen what every feature has to handle, and the kit has one author.

### Off switches

A 0.9.0 audit showed the kit could be switched off quietly. `KIT_VERIFIER=1` in the environment made every hook return 0, and a write to `.claude/settings.local.json` or `~/.zshenv` passed both guards. The same audit's probes found other commands the guards let through.

### Config

The workflow had one shape. The step pause had no config key and there were no presets. Each new behavior that gets its own block in `kit.yml` makes the file harder to read.

### Evals

The full eval baseline for 0.9.0 took about 24 minutes and $16.66 at 86 of 110 runs, which the author ruled unacceptable. A case whose with-kit and without-kit arms score the same can't show what the kit changes.

### Rules

Injected rules ([3](0003-release-0-2-ship-the-rules-through-hooks.md)) loaded out of sight, 42,919 bytes at every session and subagent start. The chat bundle already built a `kit-rules` skill ([7](0007-release-0-8-the-chat-bundle-from-the-same-files.md)), so the name and shape had precedent.

In real sessions, 40 of 168 pattern-skill loads went on to read a reference (664d02b). That commit added `reference-gate`, which denied every tool call until each file in a loaded skill's "Before | Read" table was read. 8bd6664 removed it a day later, because it loaded every reference of every loaded skill before any tool call, whatever the task.

### Claim checker

The verifier from 0.9 ([8](0008-release-0-9-verify-claims-before-they-go-out.md)) was a `claude -p` run the hook started out of sight, and a failed run was only logged. The user couldn't read what it checked or see what it cost.

### Bash edits

Since 909e23e, `output.md` sent every authored file through Write or Edit, never a shell redirect, `sed -i`, or `sd`, because only Write and Edit fired the hooks that guard, sanitize, and format written files. The ban was prose, so nothing stopped a `sed -i`, and the file it changed skipped every check. 26014ed began checking Bash edits at Stop.

### Docs for people

Release 0.4 ([5](0005-release-0-4-one-owner-per-topic-checks-from-the-project-s-own-configs.md)) keeps prose from re-describing `kit.yml` and the `kit` binary, because copies Claude read drifted (PR #8). The README never followed it for people. It described every guard, the `kit run-checks` flags, and every `kit.yml` key, and no rule, skill, or agent linked it, so Claude never read it as guidance. At 362 lines, that detail buried what the kit is for.

## Decision

### Scope

The kit isn't an autopilot. The engineer approves the plan, reviews each step, and commits. Claude stops after each plan step by default (commit eebcb53, now `step_pause` in `kit.yml`). The kit makes review cheaper and never decides what needs the engineer's judgment: it never marks a step as needing no review and never hides a change.

The kit is for one engineer on macOS or Linux. There's no team policy layer and no Windows support.

### Off switches

The guards protect the kit's own off switches. Writes to Claude Code settings files, the installed plugin root, `~/.claude/logs`, and `~/.claude/cache` ask, and `~/.zshenv` is treated like `~/.zshrc`. A guard variable is honored only with a marker named by its value's hash, with a depth counter against re-spawning. Everything else the audit's probes passed goes in a residual-risk list.

### Config

`kit.yml` stays small, and new behavior doesn't get a new block there. A preset is an overlay file under `presets/` in the same format as the user's `~/.claude/claude-kit.local.yml`, picked with `preset:` in that overlay. The merge order is `kit.yml`, then the preset, then the user's overlay, so the user's keys win. `kit config` names the file each value came from. The release ships `light`, `standard`, and `strict`.

### Evals

The 0.9.0 baseline ran once with `--ablation with-without`, and its aggregate was committed as `evals/scorecard/0.9.0.json`. Cases whose two arms scored the same were deleted. The remaining suite ran once per case, four at a time, at release only.

### Rules

The rules are the `kit-rules` skill, in `skills/kit-rules/`. A catch-all PreToolUse hook denies any tool call until the skill is loaded, for the main session and for each subagent on its own load. An agent without the Skill tool loads it by Read. The deny message is an order to load the skill and retry, because the first live test showed Claude stopping to ask instead. Hook injection is removed. Each rule is one line, a condition then an action, and its reasoning lives in `docs/rules.md`, which the model never loads.

No hook blocks on a reference read. `<required-skills>` asks for every reference of a global skill (`requiredReferences` in `go/internal/stackctx/stackctx.go`), and each skill's table asks for its own. When a file mapped to a patterns skill is read through Read, Grep with a path or glob, or a Bash command, or edited with `guard_skills` off, `skills-nudge` names the skill and its rows once per skill per session without blocking (701b343, 6b1d2b5). With `guard_skills` on, as the strict preset sets it, an edit still waits until the file's skill is loaded.

### Claim checker

The checker is `agents/claim-checker.md`, an agent anyone can read. The hook still decides when it must run, because a model choosing when to be checked would decide whether its own claims get verified. A plan, a scratch report, or a connector post is denied until the transcript shows `kit:claim-checker` ran on that exact text, the way the plan gate waits for `kit:plan-critic`. A subagent can't start an agent, so a subagent's report keeps the hook's own run, driven by the same agent file. A plan's claims are checked in `kit:plan-critic`'s run instead of a second one.

### Bash edits

`bash-edits`, a PostToolUse hook on Bash (`hooks/hooks.json`), gives every file a Bash command changed the checks an Edit gets: slop, weaken, imports, and tokens, then sanitize and format (`go/internal/hooks/bashedits.go`). A failure goes back to Claude. `output.md` drops the ban, and names a file by its absolute path whichever tool writes it. Made in 664d02b.

### Docs for people

`README.md`, `docs/guards.md`, `docs/checks.md`, and `docs/configuration.md` may explain the kit's mechanics to people. They own nothing: when one disagrees with `kit.yml` or the binary, the code wins and the page is fixed. A change to a guard, a check, or a `kit.yml` key updates the page that describes it in the same change. No rule, skill, or agent links them.

## Consequences

### Scope

- A feature that lowers review cost orders or shortens what the engineer reads. It never skips any of it. The review sheet at each pause lists every changed file, with the ones worth a look first.
- Fewer stops are the engineer's call, through `step_pause`.
- Claude never commits or pushes unasked.
- Config is one user's overlay on `kit.yml`, with no organization layer above it, and binaries ship for darwin and linux only.
- A request for a team layer or Windows support is answered with this record, until a new ADR supersedes it.

### Off switches

- Turning the kit off takes a step the user sees.
- The guards aren't a sandbox. The residual-risk list says what they don't stop.
- A request for wider bash hardening is a new decision, not a guard tweak.

### Config

- A preset file can be read in seconds and says exactly what it changes.
- A new switch is a key a preset can set, or a slug in `disabled_rules`.
- A static skill file can't follow a config key, so a rule that depends on one points to the hook line that carries it.

### Rules

- The load is visible as a tool call, and the rules cost about 28 KB, down from 43 KB.
- A reply that makes no tool call can't be gated. The session-start instruction covers it.
- A rule a hook enforces leaves the text.
- `disabled_rules: [rules-gate]` turns the gate off.
- Reading a reference is advice again, and Claude can skip it. In trigger evals of 5 runs per case, the nudge at a Read raised python loads to 4 of 5, from 8 of 15, and typescript to 5 of 5, from 5 of 15. At the edit it changed nothing (701b343).
- The light preset turns the nudge off.

### Claim checker

- The run and its cost show in the transcript like any other agent's.
- Whether Claude acted on the checker's findings is visible in the transcript and isn't enforced.
- The gate rests on undocumented Claude Code transcript formats, and the fixes that followed in 0.10 and 0.11 are in their commit bodies.

### Bash edits

- A mechanical edit can go through `sed`, a heredoc, or a script and still gets the checks.
- The checks run after the write, so a failure is sent back, where an Edit's guard stops it before.

### Docs for people

- Restated mechanics can drift across the four pages. `/audit doc-drift` finds it, as it did for 3d6eb19.
- `go/e2e/kit_repo_test.go` checks the pages' pointers as it does the README's, `docs/adr/` excepted. It doesn't check their content against `kit.yml`.
