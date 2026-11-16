# Design doc practice

## Scope

- One key proposal per doc. In doubt, split it.
- Motivation carries the weight: the user problem, the background, the concrete cases. A proposal without it gets rejected.
- Goals, and non-goals, so reviewers don't widen the work.

## Sections large projects require

- Drawbacks: the reasons not to do it.
- Alternatives: why the chosen design beats each, and the cost of doing nothing. Rejected ideas keep their reasons.
- Prior art: how others solved it, and what worked.
- Backward compatibility: each incompatibility, its severity, and how it's handled.
- Test plan and graduation criteria: what's tested at each level, and what moves the change between stages.
- Unresolved questions: those that block the merge, those that wait for implementation, and those out of scope.
- Future possibilities: what could follow. They never justify this proposal.

## Rollout and rollback

- How the change is switched on and off, and whether switching it off is safe.
- How a rollout can fail, and whether running workloads are hit.
- The metrics that trigger a rollback.
- Upgrade, downgrade, and upgrade again tested, where state or versions are involved.
- How mixed versions behave while the rollout is partial.

## Operations

- The SLIs and SLOs that show an operator the change is in use and healthy.
- What it does to call counts, object sizes, latency, and resource use.
- What happens when each dependency is down.
