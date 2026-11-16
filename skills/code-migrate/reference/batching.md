# Batching

- Parallel change has three phases. Expand adds the new form next to the old, migrate moves every caller, contract removes the old.
- Expand ships before any migrate batch, so both forms work while callers move.
- Contract is the last batch, and only once the inventory's count of old uses is zero.
- A replacement too big for one pass gets the strangler fig. New code grows beside the old, and behavior moves over one seam at a time (a module, a route, a feature). Order those batches by seam, not by file count.
- A cutover behind a release toggle gets the toggle's removal as its own late batch, since release toggles shouldn't live more than a week or two.
- Each batch leaves the system working, so it can ship alone.
