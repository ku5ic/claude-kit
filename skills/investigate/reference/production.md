# Production incidents

- Stop the bleeding first. Mitigation comes before root cause, and rollback is the generic mitigation.
- Look at changes first. Roughly 70% of outages come from a change to a live system.
- A rollback candidate is a change whose release lines up with the onset. A rollback that didn't help means the candidate was wrong, and the timeline says so.
- The incident document is living and timestamped. It can be messy but must work, and it records the ideas, the tests run, and what they showed.
- One person coordinates, and only the people operating the system change it. The user is both, so investigating stays read-only.
- Declare early. Finding a simple fix after declaring beats declaring late.
- Status updates are short and factual, on a steady cadence, each naming the next update's time.
- An external update says how customers are affected, any workaround, and the progress. No internal names. Never lie, and never guess.
- The person running the incident signs off on every update before it's posted.
