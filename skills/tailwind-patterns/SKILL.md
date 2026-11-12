---
name: tailwind-patterns
description: Tailwind CSS v4 patterns for @theme config, v3-to-v4 changes, and variants. Load before any edit to a class list, stylesheet, or tailwind.config.*, however small, and for Tailwind questions.
---

# Tailwind patterns

Default assumption: Tailwind v4 with CSS-first config.

- `@tailwindcss/postcss` or `@tailwindcss/vite` in `package.json`, or `@import "tailwindcss"` in CSS: v4.
- A `tailwind.config.js` or `.ts` and no v4 sign: v3, and the v3 lines apply.
- Both: mid-migration. Treat it as v4 and expect leftover v3 patterns.

## v4 setup

- The main CSS file: one `@import "tailwindcss"`, never the v3 `@tailwind base; @tailwind components; @tailwind utilities;` triple.
- Config: `@theme` blocks in CSS. Never create a `tailwind.config.js` in a v4 project; one is dead code unless a CSS file loads it with `@config`.
- Theme keys: `--<namespace>-<name>`, with namespaces such as `color`, `font`, `text`, `spacing`, `breakpoint`, `radius`, `shadow`, `animate`. Each key generates its utility (`--color-brand` gives `text-brand`, `--breakpoint-3xl` gives `3xl:`).
- Extending the default theme: any `--<namespace>-<name>` token in `@theme` extends it, so drop v3's `theme.extend`.
- Content paths: v4 detects them, so drop v3's `content: [...]`.
- New colors: `oklch()`, as in `--color-brand: oklch(0.7 0.15 250)`.
- Custom utility classes: `@utility`, not v3's `@layer utilities { ... }`.
- Plugins: container queries are built in; typography and forms are still separate `@tailwindcss/*` plugins. Check before installing one.

## Theme values in CSS

- Reading a theme value: `var(--color-brand)`. There's no read function in v4.
- An alpha-modified color: `--alpha(var(--color-brand) / 50%)`. v3's `theme()` helper survives in v4 only for compatibility.

## Variants

- Stacked variants in v4 apply left to right, the reverse of v3. An order-sensitive stack flips, so v3 `first:*:pt-0` becomes v4 `*:first:pt-0`.
- v4 adds `not-` (`not-hover:opacity-50`), `nth-*` (`nth-3:`, `nth-[3n+1]:`), and `starting:` for entry animations (`starting:opacity-0`). `*:` for direct children is from 3.4.
- Container queries: `@container` on the parent, `@sm:flex` and the like on children. No plugin.

## Anti-patterns

- A hardcoded hex color where a theme token would work, such as `text-[#ff0000]`: `warning`. Use the token, or add one to `@theme`.
- An `@apply` chain longer than three utilities: `warning`. Use a CSS class with raw properties, or compose in the markup.
- `dark:` everywhere when the project already has a `[data-theme="dark"]` toggle: `warning`. Pick one strategy.
- Body copy with no `text-` size: `warning`. Set the base size explicitly.
- A conditional `@apply` inside `@media` or a selector block: `info`. Inline the utilities or extract a component.
- An `animate-[...]` arbitrary value where a named animation exists: `info`. Define `--animate-<name>` in `@theme` and reuse it.
