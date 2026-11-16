# SLOs and alerts

- A service going live has at least one SLI, a measure of the service level, and an SLO, a target for it.
- Just enough SLOs to cover the service. Don't ask for one per endpoint.
- An SLO picked from current performance, with no user reason behind it, is a low finding.
- An SLO record names who approved it and when it's next reviewed, and an error budget policy names what happens when the budget runs out.
- The four golden signals are latency, traffic, errors, and saturation. A missing one is a finding, ranked by user impact.
- Alerts page on symptoms, not causes. A page for one machine's failure is too noisy to act on.
- Every page is actionable. One that merits only a robotic response shouldn't be a page.
- An SLO alerted on a static threshold alone, with no burn-rate alert (how fast the error budget is being spent), is a medium finding.
- Each alert has a playbook entry. Check the alert rule carries a link to one by the project's own key, and that the target exists.
- Playbooks go out of date as fast as production changes. A playbook older than its alert's last change is a finding.
- Error paths on the launch's request paths each log with context, count the error, or carry a trace id. How is the stack's logging and monitoring pattern skills' call.
