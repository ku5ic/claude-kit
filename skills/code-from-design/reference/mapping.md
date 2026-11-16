# Mapping

- Match each design element by role and props before name. The result is `covered` (component and props exist), `extend` (the component exists, a variant or prop doesn't), `compose` (built from existing parts), or `new`.
- Stories list the variants and states a component already has.
- Match a design value to a token by name first, then by value. A match by value alone is `likely`, for the designer to confirm.
- A token the design uses and the repo lacks goes in the Missing tokens table, with the nearest existing one.
- Layout and responsive behavior map to the repo's breakpoint tokens, not to the design's frame widths.
