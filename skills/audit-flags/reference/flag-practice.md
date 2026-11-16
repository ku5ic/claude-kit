# Flag practice

- Flags come in kinds, each with its own lifetime. Release toggles shouldn't live much longer than a week or two, experiment toggles hours or weeks, ops toggles short, and permission toggles may live for years.
- Unleash's defaults by type are 40 days for release and experiment, 7 days for operational, and permanent for kill switches and permissions. Use the service's lifetime when it reports a type.
- A permanent kind that's always on isn't stale, so class a flag before judging it.
- Flags are inventory with a carrying cost. Each stale one is a finding.
- A flag with no owner, no expiry date, and no removal task is a finding. Some teams put expiry dates on flags, or tests that fail once one passes.
- A flag that's always on leaves its other branch dead, and that branch's tests too.
- Removing one replaces the flag call with the literal it always returns, then simplifies the code around it. The finding names that literal and the branch that goes.
- The service's stale marker is a hint to review, not proof that no code reads the flag. The code decides.
- A flag that serves one variation to everyone for a long time counts as rolled out, whether that's on or off.
