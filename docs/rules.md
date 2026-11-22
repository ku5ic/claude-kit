<img src="assets/claude-kit-mark.svg" alt="" width="40">

# Why the rules say what they say

The rules in `skills/kit-rules/reference/` are written for the model: one line each, a condition then an action. This page holds the reasoning, the examples, and the exceptions that were cut from them. It's for people. The model never loads it, and a rule never cites it.

A rule a hook already enforces isn't in the rule text. The block message carries it at the moment it matters. Where that happened, the section below names the hook.

Every skill with references lists them in a "Before | Read" table: an action, then the file to read before it. In real sessions before 0.10.0, 40 of 168 pattern-skill loads went on to read a reference, because the old tables listed files by topic and nothing tied a file to a moment. A one-line rule to read the reference that fits the work failed the `ambiguous-ask` eval in 3 of 3 runs. The trigger table in kit-rules passed 3 of 3. The tables still got skipped in the session that wrote them, so 0.10.0 added `reference-gate`, which denied every tool call until each loaded skill's table files were read. It was removed on 2026-11-12. It made every reference load whatever the task, so writing a six-line bash fixture pulled in the pytest, Playwright, and React Testing Library references. Without it, `slop-comment` passed 5 of 5 runs at 0.9.0 and 2 of 3 on 2026-11-12. The skills log still records each reference read, which is how the read rate gets measured now.

## Precedence

**The kit's rules win over defaults.** Before dba9678, nothing in the system prompt put the kit's rules ahead of an output style's or the harness's own instructions. The `kit:precedence` section says the kit wins where they conflict. It sits at the top of a thread, and deep in a session the output style's per-prompt reminder is what lands next to the prompt, so the mod also puts a one-line `kit:` reminder on every prompt (db39bb3).

## Output

**Length.** Concise means a lot of information in few words. Short and missing something is wrong, and so is complete and padded. The reader has a small working memory, so one idea per line and nothing they already saw. The chat targets live in kit.yml `reply_limits.modes`, one set per `reply_length` mode, and the hooks state the turn's target on every prompt, since a line said once at session start ends up far from the reply. A hard ceiling flagged replies a word or two over (84ad86b), so a reply may now pass its target when the extra words make it clearer or a fact correct, up to the `reply_slack` allowance.

**Deliverable budgets.** A plan, a subagent report, a scratch report, or a connector post over its kit.yml `size_budgets` entry is sent back once by `go/internal/hooks/size.go`, so no rule restates the numbers. They were set against deliverables measured on 2026-11-11, where half of the plans and about a quarter of subagent handbacks ran over them. On 2026-11-16 the author doubled the plan budget and raised the report and scratch ones: a plan cut to fit leaves out what the engineer following it then has to guess, and a release plan and its explore report ran over the old numbers with nothing left to cut.

**Relayed output says whose it is, a commit doesn't.** The company page "Taking Responsibility for AI-Generated Content" asks that raw model output passed on be labelled. A commit is the engineer's reviewed work, and the commit guard blocks AI signatures, so the author kept commits unlabelled.

**What always survives.** A tradeoff that flips the decision, a risk that bites later, and a confirmation before an irreversible action are the things a reader can't recover if they're cut. A guess, a label, or a "not run" line doesn't survive: on 2026-11-21 the author asked that nothing unverified reach them, since a flagged or hedged claim hands the checking back to the reader. A check that needs a running system stays under Needs a runtime check, as a check to run.

**A command offered as the next action is run first.** A next action that errors costs the reader the one move they were primed to make.

**Where output goes.** Any tool may write a file. Since 0.10.0, `bash-edits` gives a file a Bash command changed the checks an Edit gets (slop, weaken, imports, tokens, sanitize, format), so `sed -i` no longer skips them; it can only send a failure back after the write, where an Edit's guard stops it before. A written report replaces its own summary because restating it in the terminal doubles the reading. A forked `/code-review` hands its findings to the parent as a task notification the user never sees, so the user was twice asked to decide on findings they never saw (058bf81). Returned findings go to a file first (f646fa5).

**External communication.** Someone else reads it on their time, and they usually know the project better than the writer. A generic reason ("improve readability") says nothing a reviewer can check.

## Evidence

**Check it or don't say it.** Up to 0.9.0 the rule was "check it or label it", with four confidence labels (`verified`, `likely`, `hypothesis`, `unknown`) and a provenance axis. In practice a label became a way to state an unchecked fact while looking careful, and the reader still had to do the check. A fact about the code, a version, or a result is now checked or left out. On 2026-11-21 the author dropped the last labels, since an idea offered as `likely` or `hypothesis` still handed the reader a guess to check.

**Every statement, and its supporting detail.** From the same company page. A reader of a plan or report takes each line as checked, not only the conclusion, so a wrong number or a wrong "already does" list misleads as much as a wrong conclusion.

**A result the user reports is taken as given.** Explaining why it happened, or adding caveats to it, spends the user's time on something they didn't ask about.

**Misuse or defect.** Most "library bugs" are usage errors, so the call site is the first suspect. Reading the dependency's contract (docs, types, source) settles it, memory doesn't. A known defect is often already in the tracker or changelog. Three failed fixes means the diagnosis is wrong; a fourth patch is guessing.

**A test failing after a refactor.** Adjusting the assertion to match the new output silently is the failure mode. A test asserting user-visible output is almost always guarding real behavior.

**Enforced by hooks.** The claims hook (`go/internal/hooks/claims.go`) sends back unconfirmed claims in a plan, a subagent report, an investigation, review, `audit-*`, or `code-*` report in scratch, or a connector post. A chat reply gets only the no-model pass at Stop, on its bare `file:line`, paths, SHAs, and symbols, and is corrected on the next turn (c292b51), which is why the rule itself has to hold for the rest.

## Change

**Minimal is measured against the defect, not against caution.** A one-line fix to a one-line bug is minimal. A one-line fix that papers over a broken abstraction isn't, and that's the reason to state for a bigger change. New features and requested refactors have no defect to be minimal against, hence the scope line.

**Precedent before writing.** A project's own way of solving a problem beats a generic best practice, and a change that ignores it gets reworked in review. An absence of precedent is a finding too. Not a mandate to read the codebase first: a one-line fix needs no check, and a new module, API shape, or file does. A divergence with a stated reason is a decision. One without is a defect.

**The option order** (not needed, already here, stdlib, platform, installed dependency, one line, new code) puts the cheapest maintenance first. CSS over JS and a DB constraint over app code are the platform cases.

**Blast radius.** A one-line change to a widely imported module is a wide change wearing a small diff. Consumers that can't be listed quickly are a risk the reviewer should hear about before the edit.

**Dead code.** Version control holds the history, so commented-out code only costs readers.

**Comments.** If removing a comment wouldn't confuse a future reader, it shouldn't exist. A "why" that needs a paragraph belongs in the commit message. `shortcut:` comments are collected by `/audit-debt`.

**Formatting** left the rule because each write is formatted by the project's fixers, or by kit.yml `on_edit` where none claims the file.

**Principles.** KISS, YAGNI, DRY, and SOLID are enforced by the judgment each turns on, not by the acronym. A preference dressed as a principle isn't a finding. DRY's exception (copies with divergent lifecycles) exists because merging them couples things that change for different reasons.

**Never simplified away.** Input validation, data-loss error handling, security controls, and accessibility look like excess to a simplifier and aren't.

## Verify

**Hooks run the gates, Claude runs them on request.** The Stop hook checks a turn's edits, and the `/code-review` hook runs the full gate. Each shows the user a receipt. Claude runs `kit run-checks` only when the user asks for the gate, because a full run can take minutes. Repeating verdicts in a reply only doubles what the user already saw.

**Tests aren't the runtime pass.** Tests prove the asserted paths pass, not that the change works for whoever consumes it. A pure refactor is covered by its tests, except in UI, where tests don't assert the look.

**A FAIL needs 90% confidence to become a finding** because a review full of false positives trains the reader to skip findings.

**Starting it and seeing it respond isn't verification.** The axis table exists because the happy path is the one case that was already tried.

**Upstream gaps.** A case the contract doesn't define belongs to its owner. Patching around it quietly hides the question from the one person who can answer it.

**One cycle, then stop.** Without a stopping rule, review loops find ever-smaller issues and never end. A regression against the base branch is the line between "fix now" and "follow-up".

**The last plan step** is held by the Stop hook until a review started after the last edit has finished.

## Workflow

**Enforced by hooks.** The bash guard blocks a push to kit.yml `protected_branches`, and asks before a commit the current prompt doesn't ask for, an add of a package, `git reset --hard`, and `git clean`. The attribution guard blocks AI signatures and "Generated by" footers in commits, tags, and GitHub posts. The protected-branch line left the rule. The signature line left too and came back in 0.10.0: a commit request then got a proposed message first, which no commit hook sees, and the eval showed proposals carrying Claude Code's default co-author line. A commit request is now the go itself, and the message and diff summary follow the commit. The rest stay, since the guard doesn't cover every case the rule lists.

**Mutating commands in their own call.** A failure earlier in a chain can mask whether the mutation ran. Each segment of a chain is still checked by the guard.

**Plans change on the engineer's word.** Plan mode is for discussion. A plan edited on every passing idea drifts from what was agreed, and the plan-changes hook prints a change list on resubmit so the engineer sees exactly what moved. An answer to a question about the plan is the engineer's word too: left out of the plan file, it makes the plan stale, and the step-pause review sheet flags each file the plan doesn't name (`lookReasons` in `go/internal/hooks/reviewsheet.go`).

**Procedure skills assume nothing and do their own prerequisites.** A skill that names one stack, host, or vendor fails silently everywhere else, so it detects them and takes specifics from the pattern skills. A skill that answers "run `/code-visual` first" sends the user off to type a second command for output it could build itself (`/code-tour` did, before 0.12.0). The exception is input the user has to vet, such as a draft to post.

**Questions are batched.** A question in the middle of a step interrupts work the engineer thought was running. Batching at plan time and at pauses, with a recommended option, turns each answer into a quick decision that's recorded and never asked again.

**The reason comes from the engineer.** A reason the model invents reads plausible and may be wrong, and it ends up in commit messages and PR descriptions under the engineer's name.

**Ask or proceed.** A question answered with an edit surprises the user. A request with two readings acted on with a guess wastes the edit half the time. A vague goal counts as ambiguous even with an obvious bug in the code, because the bug is one of the options, not the answer.

**Review findings are claims.** A reviewer can be wrong, and an applied wrong finding is a regression with a reviewer's name on it.

**A review or runtime pass stops at its report.** Fixes applied before the report exists leave the engineer judging findings that are already in the tree, and the report then describes code that changed under it. On 2026-11-20 a step's six findings were fixed before their report and before the engineer chose any.

## Subagents

**Spawning costs.** Each agent has its own request budget, and its report is a handoff that loses detail. Doing the work directly is cheaper unless the task needs isolation or parallelism.

**The model fits the task.** An agent with no `model:` inherits the session's. In the 0.10.0 skill rewrite, template work ran on Opus at over 100k tokens per agent. Diff review went to Opus too, as "judgment": four `/simplify` cleanup reviewers ran on it at about 80k tokens each, and Claude Code's usage panel named general-purpose subagents as the top cost. A diff review checks code against a brief, which Sonnet does; a plan or design critique weighs choices nobody has made yet, which is where Opus earns its price.

**An agent's report describes intent.** It says what the agent meant to do, not necessarily what it did, so edits get a diff and findings get a spot check before anything rests on them. Two agents disagreeing means neither is correct until traced.

## Tooling

**CLI first.** One deterministic CLI call uses fewer tokens than reading files and reasoning, and its answer doesn't drift.

**A missing tool skips a step, not the task.** Most of the kit's tools aren't preinstalled ([dependencies](dependencies.md)). A skill that stopped on the first one missing gave the user nothing, so the step goes, the report names it, and the rest still runs.

**References are outside the read budget.** A skill's SKILL.md is a table of pointers, and the rules live in the references it links. A model that applies the budget rules to them reads the table, skips the rules, and follows the skill in name only.

**Bare `kit`.** Permission allows are written against the bare command, so a pathful or wrapped call misses them and prompts.

**Never the project root.** A stray file pollutes `git status`, risks being committed, and lands in every clone. The bash guard asks before a write into the current directory, and the download guard keeps `curl` and `wget` output in scratch.
