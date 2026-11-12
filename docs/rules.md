# Why the rules say what they say

The rules in `skills/kit-rules/reference/` are written for the model: one line each, a condition then an action. This page holds the reasoning, the examples, and the exceptions that were cut from them. It's for people. The model never loads it, and a rule never cites it.

A rule a hook already enforces isn't in the rule text. The block message carries it at the moment it matters. Where that happened, the section below names the hook.

Every skill with references lists them in a "Before | Read" table: an action, then the file to read before it. In real sessions before 0.10.0, 40 of 168 pattern-skill loads went on to read a reference, because the old tables listed files by topic and nothing tied a file to a moment. A one-line rule to read the reference that fits the work failed the `ambiguous-ask` eval in 3 of 3 runs. The trigger table in kit-rules passed 3 of 3. The tables still got skipped in the session that wrote them, so 0.10.0 added `reference-gate`, which denied every tool call until each loaded skill's table files were read. It was removed on 2026-11-12. It made every reference load whatever the task, so writing a six-line bash fixture pulled in the pytest, Playwright, and React Testing Library references. Without it, `slop-comment` passed 5 of 5 runs at 0.9.0 and 2 of 3 on 2026-11-12. The skills log still records each reference read, which is how the read rate gets measured now.

## Output

**Length.** Concise means a lot of information in few words. Short and missing something is wrong, and so is complete and padded. The reader has a small working memory, so one idea per line and nothing they already saw. The chat ceilings live in kit.yml `reply_limits`, and the hooks state the chat ceiling once at session start and a turn's own only when its prompt sets another, so the rule only says to stay under it.

**Deliverable budgets.** A plan, a subagent report, a scratch report, or a connector post over its kit.yml `size_budgets` entry is sent back once by `go/internal/hooks/size.go`, so no rule restates the numbers. They were set against deliverables measured on 2026-11-11, where half of the plans and about a quarter of subagent handbacks ran over them.

**Relayed output says whose it is, a commit doesn't.** The company page "Taking Responsibility for AI-Generated Content" asks that raw model output passed on be labelled. A commit is the engineer's reviewed work, and the commit guard blocks AI signatures, so the author kept commits unlabelled.

**What always survives.** A tradeoff that flips the decision, a risk that bites later, and a confirmation before an irreversible action are the things a reader can't recover if they're cut. What wasn't run stays as one line because a missing line reads as "it was run".

**A command offered as the next action is run first.** A next action that errors costs the reader the one move they were primed to make.

**Where output goes.** Any tool may write a file. Since 0.10.0, `bash-edits` gives a file a Bash command changed the checks an Edit gets (slop, weaken, imports, tokens, sanitize, format), so `sed -i` no longer skips them; it can only send a failure back after the write, where an Edit's guard stops it before. A written report replaces its own summary because restating it in the terminal doubles the reading.

**External communication.** Someone else reads it on their time, and they usually know the project better than the writer. A generic reason ("improve readability") says nothing a reviewer can check.

## Evidence

**Check it or don't say it.** Up to 0.9.0 the rule was "check it or label it", with four confidence labels (`verified`, `likely`, `hypothesis`, `unknown`) and a provenance axis. In practice a label became a way to state an unchecked fact while looking careful, and the reader still had to do the check. A fact about the code, a version, or a result is now checked or left out. The labels stay only for an idea the writer offers as an idea, where the label tells the reader it's a guess to test, not a finding.

**Every statement, and its supporting detail.** From the same company page. A reader of a plan or report takes each line as checked, not only the conclusion, so a wrong number or a wrong "already does" list misleads as much as a wrong conclusion.

**A result the user reports is taken as given.** Explaining why it happened, or adding caveats to it, spends the user's time on something they didn't ask about.

**Precedent before writing.** A project's own way of solving a problem beats a generic best practice, and a change that ignores it gets reworked in review. An absence of precedent is a finding too. Not a mandate to read the codebase first: a one-line fix needs no check, and a new module, API shape, or file does. A divergence with a stated reason is a decision. One without is a defect.

**Misuse or defect.** Most "library bugs" are usage errors, so the call site is the first suspect. Reading the dependency's contract (docs, types, source) settles it, memory doesn't. A known defect is often already in the tracker or changelog. Three failed fixes means the diagnosis is wrong; a fourth patch is guessing.

**A test failing after a refactor.** Adjusting the assertion to match the new output silently is the failure mode. A test asserting user-visible output is almost always guarding real behavior.

**Enforced by hooks.** The claims hook (`go/internal/hooks/claims.go`) sends back unconfirmed claims in a plan, a subagent report, a scratch report, or a connector post. A chat reply is never checked, which is why the rule itself has to hold there.

## Change

**Minimal is measured against the defect, not against caution.** A one-line fix to a one-line bug is minimal. A one-line fix that papers over a broken abstraction isn't, and that's the reason to state for a bigger change. New features and requested refactors have no defect to be minimal against, hence the scope line.

**The option order** (not needed, already here, stdlib, platform, installed dependency, one line, new code) puts the cheapest maintenance first. CSS over JS and a DB constraint over app code are the platform cases.

**Blast radius.** A one-line change to a widely imported module is a wide change wearing a small diff. Consumers that can't be listed quickly are a risk the reviewer should hear about before the edit.

**Dead code.** Version control holds the history, so commented-out code only costs readers.

**Comments.** If removing a comment wouldn't confuse a future reader, it shouldn't exist. A "why" that needs a paragraph belongs in the commit message. `shortcut:` comments are collected by `/audit debt`.

**Formatting** left the rule because the `on_edit` formatters in kit.yml apply it on every write.

**Principles.** KISS, YAGNI, DRY, and SOLID are enforced by the judgment each turns on, not by the acronym. A preference dressed as a principle isn't a finding. DRY's exception (copies with divergent lifecycles) exists because merging them couples things that change for different reasons.

**Never simplified away.** Input validation, data-loss error handling, security controls, and accessibility look like excess to a simplifier and aren't.

## Verify

**Hooks run the gates, Claude doesn't.** The Stop hook checks a turn's edits, and the `/code-review` hook runs the full gate. Each shows the user a receipt. The bash guard blocks `kit run-checks`, so the rule no longer says so. Repeating verdicts in a reply only doubles what the user already saw.

**Tests aren't the runtime pass.** Tests prove the asserted paths pass, not that the change works for whoever consumes it. A pure refactor is covered by its tests, except in UI, where tests don't assert the look.

**A FAIL needs 90% confidence to become a finding** because a review full of false positives trains the reader to skip findings.

**Starting it and seeing it respond isn't verification.** The axis table exists because the happy path is the one case that was already tried.

**Upstream gaps.** A case the contract doesn't define belongs to its owner. Patching around it quietly hides the question from the one person who can answer it.

**One cycle, then stop.** Without a stopping rule, review loops find ever-smaller issues and never end. A regression against the base branch is the line between "fix now" and "follow-up".

**The last plan step** is held by the Stop hook until a review started after the last edit has finished.

## Workflow

**Enforced by hooks.** The bash guard blocks a push to kit.yml `protected_branches`, and asks before a commit, an install, `git reset --hard`, and `git clean`. The commit guard blocks AI signatures and "Generated by" footers. The protected-branch line left the rule. The signature line left too and came back in 0.10.0: a commit request now gets a proposed message first, which no commit hook sees, and the eval showed proposals carrying Claude Code's default co-author line. The rest stay, since the guard doesn't cover every case the rule lists.

**Mutating commands in their own call.** A failure earlier in a chain can mask whether the mutation ran. Each segment of a chain is still checked by the guard.

**Plans change on the engineer's word.** Plan mode is for discussion. A plan edited on every passing idea drifts from what was agreed, and the plan-changes hook prints a change list on resubmit so the engineer sees exactly what moved.

**Questions are batched.** A question in the middle of a step interrupts work the engineer thought was running. Batching at plan time and at pauses, with a recommended option, turns each answer into a quick decision that's recorded and never asked again.

**The reason comes from the engineer.** A reason the model invents reads plausible and may be wrong, and it ends up in commit messages and PR descriptions under the engineer's name.

**Ask or proceed.** A question answered with an edit surprises the user. A request with two readings acted on with a guess wastes the edit half the time. A vague goal counts as ambiguous even with an obvious bug in the code, because the bug is one of the options, not the answer.

**Review findings are claims.** A reviewer can be wrong, and an applied wrong finding is a regression with a reviewer's name on it.

## Subagents

**Spawning costs.** Each agent has its own request budget, and its report is a handoff that loses detail. Doing the work directly is cheaper unless the task needs isolation or parallelism.

**The model fits the task.** An agent with no `model:` inherits the session's. In the 0.10.0 skill rewrite, template work ran on Opus at over 100k tokens per agent.

**An agent's report describes intent.** It says what the agent meant to do, not necessarily what it did, so edits get a diff and findings get a spot check before anything rests on them. Two agents disagreeing means neither is correct until traced.

## Tooling

**CLI first.** One deterministic CLI call uses fewer tokens than reading files and reasoning, and its answer doesn't drift.

**References are outside the read budget.** A skill's SKILL.md is a table of pointers, and the rules live in the references it links. A model that applies the budget rules to them reads the table, skips the rules, and follows the skill in name only.

**Bare `kit`.** Permission allows are written against the bare command, so a pathful or wrapped call misses them and prompts.

**Never the project root.** A stray file pollutes `git status`, risks being committed, and lands in every clone. The bash guard asks before a write into the current directory, and the download guard keeps `curl` and `wget` output in scratch.
