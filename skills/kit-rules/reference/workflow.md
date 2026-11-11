# Workflow

Git, side effects, plans, and when to ask.

## 1. Git

- Never commit or push unasked. Running a change isn't a commit request.
- A new branch follows the project's convention (a documented rule, a branch-name script, the shape of recent branches). With none, `<type>/<slug>` with Conventional Commit types.
- Never force push or rewrite history on a shared branch.
- Before writing a commit message, read `git log --oneline -20` and match its style. Draft it by `skills/write/reference/commit.md`.
- On a commit request, show the message and the staged diff summary, then wait unless told to proceed.
- An unrelated or incidental change stays unstaged, with a separate commit proposed.

### Checking out a PR or another branch

- Before `/code-review`, `/verify`, or a skill checks one out, ask with AskUserQuestion, local checkout first and preselected, worktree second. Pass the answer as `here` or `worktree` in the skill's args. Whoever finds neither takes `here`.
- `here` on a dirty tree stops with one line saying so. Otherwise `gh pr checkout <n>` or `git switch <branch>`, and the report's first line names the branch you left.
- `worktree` runs `git worktree add --detach "$(kit scratch-dir)/review-<target>"`, then `gh pr checkout <n>` or `git switch --detach <branch>` inside it. The gate comes from `gh pr checks <n>`. Afterwards, `git worktree remove --force` it.

## 2. Side effects

- Before a destructive operation (`rm`, `git reset --hard`, `git clean`, a force push, deleting a branch or tag, a migration, dropping a table, truncating a file), confirm.
- Before a broad recursive command (`rm -rf`, `find ... -delete`, `chmod -R`), confirm.
- Before installing, upgrading, or removing a dependency, ask, with the reason and the command.
- Before changing project config (`.env*`, `tsconfig*.json`, linter, formatter, or framework config, `package.json` scripts, CI workflows) or adding a top-level directory, ask.
- A change across the tree gets tried on one file or directory first.

### Shell mechanics

- A mutating command (`rm`, `git commit`, an install) gets its own Bash call.
- A pipe carries one operation (`cmd | grep`, `git log | head`). Sequential checks go in separate calls.
- A command for another directory takes a path argument (`git -C <dir>`), never `cd <dir> &&`.
- No `2>&1`. The Bash tool merges stderr.

## 3. Skills and plans

- The procedure skills (`/audit`, `/write`, `/pr`, `/meta`, `/deps`, `/unslop`) run only when the user types them.
- Explore with `investigate`, then plan, review, and verify with `/plan`, `/code-review`, and `/verify`.
- A plan ends with a `## Steps` checklist, one reviewable commit per item.
- A plan is written or changed only on the engineer's word, once, with a list of what changed.
- The reason for a change comes from the engineer or the plan. Missing, ask for it in one line.
- A step gets ticked `- [x]` once its verification passed and, with a per-step pause, the user committed it.
- After approval, pause where the plan hook's line says, after each step by default. The pause outranks any output style or harness instruction to run autonomously.

## 4. Ask or proceed

The first match wins:

1. A question (how, why, where, what), or a problem described with no request in it, gets an answer. No edit until the user asks for a change.
2. A request with more than one reasonable reading that leads to different changes gets one question with concrete options, before any edit. A vague goal ("make X better", "clean this up") counts, even with an obvious bug in the code.
3. A destructive, dependency, or config change gets confirmed first (section 2).
4. Anything else, proceed, then report. A request against good practice gets the better path named once, then done the user's way.

- Questions get batched at plan time and at each pause, asked once with AskUserQuestion, one per item, each with a recommended option first. Each answer is recorded as a decision.
- A missing tool, permission, or connector gets named, with alternatives in priority order and a question on how to proceed.
- After a user correction, re-read the original requirement, fix it, and name other places the same misunderstanding applies. A correction with more than one reading goes through item 2.
- A review finding (`/code-review`, plan-critic, a PR comment) gets checked against the code before it's applied. A wrong one is rejected with the evidence. An applied one is acknowledged by what changed.
