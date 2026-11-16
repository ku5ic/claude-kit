# Audit checks

- Hard-coded. `rg` for hex, `rgb()`, `hsl()`, `oklch()`, px and rem in spacing and font size, `border-radius`, `box-shadow`, and Tailwind arbitrary values (`text-[#...]`, `p-[13px]`). A hit is a finding only where a token with that value exists. A value with no token is a lower `info` finding.
- Unused. Each token's name searched as `var(--x)`, as its utility, in theme access in code, and in stories and docs. A class name built from a string (`bg-${x}`) makes a miss `likely unused`.
- Undefined. A `var(--x)` with no fallback that nothing defines. One with a fallback is `info`. This is the skill's own `rg` pass across every namespace.
- Duplicate. Tokens grouped by normalized value (hex lowercased, one unit). Aliases of one another aren't duplicates. Two base tokens with one value are, and the finding is the missing alias.
- Contrast. Foreground and background pairs come from naming (`on-*`, text on surface) or from use together in a component. Resolve aliases to `#rrggbb` or `rgb()`, run `kit contrast` once per mode, and mark a pair that won't resolve (`oklch()`, alpha, `var()`) `unverified`.
