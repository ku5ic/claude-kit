# Incident practice

- Blameless: name the contributing causes in systems and processes, and assume each person acted well on what they knew. The setup that made the mistake easy gets fixed.
- Worth a write-up: user-visible downtime or degradation past an agreed threshold, any data loss, an on-call intervention such as a rollback, a long time to resolve, or a failure found by a person instead of monitoring. The thresholds are agreed before an incident.
- Contents: the incident and its impact, the mitigation, the root causes understood in depth, the follow-up actions, and the data a later reader will want.
- Action items favor prevention.
- A senior reader reviews it before it's final. An unreviewed postmortem might as well not exist.
- Store it where the team can search it.
- Write it soon, while the details are fresh.
- Every action item is filed as a ticket. Without tracking, action items get forgotten and the outage repeats.
- A postmortem with no action after it is the same as none. A rerun checks each item's closure and how long it's been open.
- Failures that mirror an earlier incident mean digging deeper. The retro records a failure signature (the alert name, the service, the top error with ids and numbers stripped) so a rerun can search for it. The signature's shape is the kit's own, since machine-readable tags are the nearest thing the sources describe.
- The review asks whether the key data was kept, whether the root cause went deep enough, and whether the action plan and its priorities fit.
