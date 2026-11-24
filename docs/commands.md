<img src="assets/claude-kit-mark.svg" alt="" width="40">

# Commands and agents

Each command is a procedure Claude follows step by step, so the same request gets the same shape of answer every time. Only you start one, by typing it. A command may run another's procedure as one of its steps. Each is its own entry in the `/` menu, so typing `/audit` lists every audit with what it does. They work bare (`/audit-perf`) or namespaced (`/kit:audit-perf`).

Every command detects your stack, your code host (GitHub through `gh`, GitLab through `glab`, else the host's MCP connector), and the tools you've connected, and asks when it can't tell. Audit and code commands are read-only on your repo: each writes a report to `.claude/scratch/`, and with the `strict` preset the claim checker checks it before you see it. An audit then offers the read-only checks that need a running system, such as `tofu plan` or axe on a running page, and runs the ones you pick.

`investigate` isn't on this list because Claude loads it on its own when you ask how, why, or where something works: [docs/skills.md](skills.md).

## Audit: a read-only report

| Command            | What it does                                                                                                      | Use it when                                                            |
| ------------------ | ----------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `/audit-a11y`      | WCAG 2.2 AA audit of a component, page, or template                                                               | Before shipping UI, or after an accessibility complaint                |
| `/audit-arch`      | Judges the code's structure against the architecture the project says it has                                      | Layers or modules seem to leak into each other                         |
| `/audit-ci`        | Hardening review of CI workflow files                                                                             | Adding or changing a workflow, or reviewing CI permissions and secrets |
| `/audit-debt`      | Technical debt inside a target's modules, ranked, with a remediation path                                         | Deciding what to clean up before or during feature work                |
| `/audit-doc-drift` | Where docs, ADRs, or RFCs no longer match the code                                                                | Before a release, or after a refactor that touched documented code     |
| `/audit-flags`     | Stale feature flags, flags the code and the flag service disagree on, missing owners or expiry, the dead branches | Flag count keeps growing, or before a cleanup sprint                   |
| `/audit-infra`     | Review of Docker, Terraform or OpenTofu, and server provisioning files                                            | Changing infra, or inheriting it                                       |
| `/audit-perf`      | Costs a static read can find, then a runtime measurement after your yes                                           | Something is slow and you don't know where                             |
| `/audit-readiness` | Go or no-go before a launch: owner, SLOs, alerts with runbooks, rollback, flags, dependency failure modes         | Before a launch or a first production deploy                           |
| `/audit-security`  | Security review of the repo, a directory, or a file                                                               | Before a release, or for code handling auth, secrets, or user input    |
| `/audit-slop`      | AI tells and filler across a repo's prose, worst files first                                                      | Docs or comments read like generated text                              |
| `/audit-tests`     | Tests that can't fail, test the implementation, duplicate another, or cover nothing worth testing                 | A suite grows slower and bigger but bugs still get through             |
| `/audit-tokens`    | Hard-coded values a design token covers, unused, undefined, and duplicate tokens, and token contrast              | A design system drifts from its tokens                                 |
| `/audit-verify`    | Re-checks a prior audit report's findings against the repo as it is now                                           | After fixing an audit's findings                                       |

## Code: understand a change before making it

| Command             | What it does                                                                                       | Use it when                                             |
| ------------------- | -------------------------------------------------------------------------------------------------- | ------------------------------------------------------- |
| `/code-challenge`   | Challenges a plan's, a PR's, or a design doc's decisions the way a senior reviewer would           | Before approving a design, yours or someone else's      |
| `/code-contract`    | What a change does to a public surface, each change classed patch, minor, or major, with consumers | Changing an API, a schema, or an exported type          |
| `/code-deep-review` | A deep review of a path or change: design, precedent, test strength, error paths, security         | A change too big or risky for a quick `/code-review`    |
| `/code-evaluate`    | Whether to adopt a package, or which of several: health, security, license, footprint, exit cost   | Before adding a dependency                              |
| `/code-from-design` | Maps a design to your components and tokens, with what's missing and the states the design omits   | Starting UI work from a Figma or other design file      |
| `/code-history`     | A path's git and PR history: turning points, recurring bugs, who to ask                            | Before changing code you didn't write                   |
| `/code-impact`      | What breaks if a file or symbol changes: consumers, their tests, their churn                       | Before a refactor or a rename                           |
| `/code-migrate`     | Every use of what a migration replaces, in one repo or across an org, batched into plan steps      | Replacing a library, an API, or a pattern everywhere    |
| `/code-tour`        | An onboarding walk: where the code starts, one path end to end, where it changes, who to ask       | New to a repo                                           |
| `/code-visual`      | Mermaid diagrams from the code's size, imports, churn, and owners, each with its takeaway          | You want the architecture as a picture, or its hotspots |

## Write: a document in your project's format

| Command                | What it does                                                                                  | Use it when                                     |
| ---------------------- | --------------------------------------------------------------------------------------------- | ----------------------------------------------- |
| `/write-adr`           | An architecture decision record in the project's own format, each sentence traced to a source | A decision was made and should outlast the chat |
| `/write-commit`        | A commit message from the staged diff, in the project's style                                 | Committing                                      |
| `/write-devnote`       | A sentence or two on a change's key reasoning, or on what testing someone's change found      | A PR or ticket needs the why, briefly           |
| `/write-explainer`     | A bug, feature, or module explained to someone new, with `file:line` citations                | Handing work over, or onboarding someone        |
| `/write-launch`        | An announcement post, landing page copy, or the facts for a Show HN you write yourself        | Launching a project, a release, or a feature    |
| `/write-release-notes` | Release notes from a branch's commits since its base                                          | Cutting a release                               |
| `/write-retro`         | A retro for an incident, sprint, or feature, its action items filed as tickets after your yes | After an incident or at the end of a sprint     |
| `/write-rfc`           | A design doc whose current state, rollout, rollback, and capacity come from code and metrics  | Proposing a change others have to agree to      |
| `/write-runbook`       | A runbook for an alert or a service, each step checked against the repo                       | An alert has no runbook, or its runbook is old  |
| `/write-stakeholder`   | A technical finding or proposal reframed for a non-technical reader                           | Reporting up or across                          |
| `/write-tickets`       | Tickets from a plan, a report, or a description, posted to your tracker after your yes        | Turning a plan or audit into tracked work       |

## PR: your PRs and other people's

| Command           | What it does                                                                           | Use it when                            |
| ----------------- | -------------------------------------------------------------------------------------- | -------------------------------------- |
| `/pr-description` | Your PR's title and body from the branch's diff, in the project's template             | Opening a PR                           |
| `/pr-queue`       | PRs waiting on your review, and your own PRs needing action, ranked                    | Starting the day, or between tasks     |
| `/pr-reply`       | Draft replies to the review on your PR, each claim checked against the code            | Your PR got review comments            |
| `/pr-review`      | This session's review findings on someone else's PR as a draft review, inline comments | You reviewed a PR and want to post it  |
| `/pr-publish`     | Posts an approved draft: a review, replies, or a description                           | You've read a draft and want it posted |

Nothing is posted until you run `/pr-publish` on a draft you've read.

## Meta: the kit and your sessions

| Command             | What it does                                                                                                                                                                                             | Use it when                                                        |
| ------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------ |
| `/meta-conventions` | Writes down the conventions a repo follows, as path-scoped rules with a citation for each                                                                                                                | Claude keeps missing a repo's unwritten rules                      |
| `/meta-prompt`      | Turns a fuzzy ask into a sharp prompt with context and acceptance criteria                                                                                                                               | Before a big task you can't yet state precisely                    |
| `/meta-refresh`     | Checks the skills' sources against current releases and updates their guidance                                                                                                                           | A library the skills cover had a major release                     |
| `/meta-retro`       | A retro on Claude Code sessions, one or many, each finding cited to the transcript                                                                                                                       | A session went wrong, or the same thing keeps going wrong          |
| `/meta-skill`       | Drafts a pattern skill for a stack from current docs and the repo's precedent                                                                                                                            | Your stack has no pattern skill                                    |
| `/kit-stats`        | Prints `kit stats` from the kit's logs: blocks, shadow blocks, and skips per rule, and the claims each checker flagged. A mod runs it, with no turn for Claude, and the `kit_stats` setting turns it off | You want to see how often the kit stepped in, or what a rule costs |

## Others

| Command   | What it does                                                                | Use it when                             |
| --------- | --------------------------------------------------------------------------- | --------------------------------------- |
| `/deps`   | Merges Dependabot or Renovate PRs and reconciles the host's security alerts | Dependency PRs and alerts have piled up |
| `/unslop` | Rewrites AI tells and filler in a file, a diff, or pasted text              | A text reads like it was generated      |

## Agents

Each agent is a subagent with one job, the tools for it, and a boundary it keeps: the debugger hands the fix back instead of applying it, and the tester never changes the code to make a test pass. Claude starts them itself, or you can ask for one by name.

| When you need                                      | Agent               | What it does                                                                                                                                                 |
| -------------------------------------------------- | ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Most `audit-*` commands                            | `kit:auditor`       | Runs the audit read-only and writes the report                                                                                                               |
| To find where and why something breaks             | `kit:debugger`      | Localizes the fault with at most one probe edit. Hands the fix back                                                                                          |
| One fact: precedent, tests, or history             | `kit:gatherer`      | Answers with `file:line` only, never opinions                                                                                                                |
| A plan attacked before you approve it              | `kit:plan-critic`   | Reads the plan against the real repo, sending its fact questions to `kit:gatherer` runs in parallel                                                          |
| A text's claims checked before anyone relies on it | `kit:claim-checker` | Reports each claim nothing confirms, with the check that would settle it. Run when you ask for a fact-check; the claims hook uses its prompt for its own run |
| Library docs or a web page                         | `kit:researcher`    | Looks them up, keeping network access out of code work                                                                                                       |
| Tests for recent work                              | `kit:tester`        | Adds or updates them and runs them. Never changes the code to make them pass                                                                                 |
