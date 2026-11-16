# Runbook practice

- An alert requires a person to act. Its runbook gives clear steps to resolve the problem, and an alert with neither steps nor a runbook is useless.
- Each alert has one runbook entry.
- Sections, in order. What the alert means and the metric that triggers it, its severity and user impact, triage steps, mitigations (rollback, flag off, drain), who to escalate to and when, and links to the dashboards and the alert rule.
- A runbook gives high-level instructions, explains the alert's severity and impact, and suggests where to debug.
- Runbooks cut the time to repair. Google reports roughly 3x against working it out on the spot.
- Runbooks go out of date as fast as production changes. The footer asks whoever is paged to update it when the alert fires, and shows the date it was last checked.
- A step people keep running by hand is a candidate for automation or a console. Name it, don't build it.
