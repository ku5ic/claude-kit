---
name: tailwind-patterns
description: Tailwind CSS v4 patterns - CSS-first config via the @theme directive, v3-vs-v4 deltas, variant rules, and anti-patterns. Use whenever the project contains a `tailwind.config.*`, `@import "tailwindcss"` in CSS, or a `@tailwindcss/*` package dependency, OR the user asks about Tailwind, utility classes, design tokens, or dark-mode strategy in a Tailwind project, even if Tailwind is not mentioned by name.
---

# Tailwind patterns

Default assumption in this dotfiles project: Tailwind v4 with CSS-first config. If the project has a `tailwind.config.js` it is v3 and the v3 section applies; otherwise v4.

## v4 vs v3 at a glance

- v4 setup: a single `@import "tailwindcss"` in the main CSS file. No `@tailwind base / components / utilities` directives.
- v4 config: lives in CSS via the `@theme` directive. No `tailwind.config.js`. Theme tokens become CSS custom properties automatically.
- v4 content detection: automatic. The compiler scans the project for class names; you do not configure `content` paths.
- v4 plugins: still work. Container queries are built in; typography and forms remain separate `@tailwindcss/*` plugins. Check before installing.
- v4 build: ~5x faster. The Oxide engine replaces PostCSS for the core compile step.

## Setting up v4 correctly

```css
/* app/globals.css or equivalent */
@import "tailwindcss";

@theme {
  --color-brand: oklch(0.7 0.15 250);
  --font-display: "Inter", sans-serif;
  --spacing-page: 4rem;
  --breakpoint-3xl: 1920px;
}
```

- Theme keys follow the pattern `--<namespace>-<name>`. Namespaces: `color`, `font`, `text`, `spacing`, `breakpoint`, `radius`, `shadow`, `animate`, etc.
- Defining a key generates the corresponding utility (`text-brand`, `font-display`, `p-page`, `3xl:`).
- Use `oklch()` for new colors; v4's default palette is OKLCH-based for better wide-gamut support.

## v3 patterns to avoid in v4

- `tailwind.config.js`: do not create one in a v4 project. Move config into `@theme` blocks. If the file exists in a v4 project, it is dead code.
- `@tailwind base; @tailwind components; @tailwind utilities;`: v3 directive triple. v4 replaces these with a single `@import "tailwindcss"`.
- `theme.extend`: v3 JS shape. In v4, all CSS-defined `--<namespace>-<name>` tokens extend the default theme automatically.
- `content: [...]`: v3 path config. v4 auto-detects.
- `@layer utilities { ... }` for arbitrary utility classes: still works, but prefer defining custom utilities with the `@utility` directive in v4 for better tree-shaking.

## Using theme values from CSS

- `var(--color-brand)`: read a theme value in plain CSS. Every `@theme` key is a CSS custom property; there is no separate read function in the v4 docs.
- `--alpha(var(--color-brand) / 50%)`: compute an alpha-modified color in v4. v3 used the `theme()` helper, kept in v4 only for compatibility.

## Variants and modifiers

- v4 variant order: `hover:focus:bg-blue-500` applies hover AND focus, just like v3. No change.
- New in v4:
  - `not-` variant prefix (`not-hover:opacity-50`)
  - `nth-child(n)` shorthand (`*:rounded-md`)
  - starting-style variants (`starting:opacity-0` for entry animations)
- Container queries are built in: `@container` on the parent, `@sm:flex` etc. on children. No plugin install.

## Anti-patterns to flag in review

**warning: hardcoded hex colors when a theme token would work**
`text-[#ff0000]` bypasses the design system. Use the existing token; if missing, add it to `@theme`.

**warning: `@apply` chains longer than three utilities**
They obscure the cascade and inhibit tree-shaking. Use a CSS class with raw properties instead, or compose at the markup level.

**info: conditional `@apply` inside `@media` or selector blocks**
Works, but readability falls off fast. Inline utilities or extract a component.

**warning: `dark:` everywhere instead of a `[data-theme="dark"]` strategy**
Inconsistent when the project already has a theme toggle. v4 supports both; pick one.

**info: `animate-[...]` arbitrary values when a named animation exists**
Define `--animate-<name>` in `@theme` and reuse it instead of repeating the arbitrary value.

**warning: missing `text-` size on body copy**
Relying on the browser default (16px) instead of setting the base size explicitly leaves typography undocumented and inconsistent across components.

## Verifying which version is in use

```sh
# v4: postcss config has @tailwindcss/postcss, OR vite plugin @tailwindcss/vite
rg '@tailwindcss/(postcss|vite)' package.json

# v4 config in CSS
rg '@import "tailwindcss"' --type css

# v3 sentinel: a tailwind.config.js or .ts
fd -t f 'tailwind.config'
```

If both `tailwind.config.js` and `@import "tailwindcss"` are present, the project is mid-migration. Treat as v4 but expect leftover v3 patterns.

## References

- v4 release post: https://tailwindcss.com/blog/tailwindcss-v4
- v3-to-v4 upgrade guide: https://tailwindcss.com/docs/upgrade-guide
- v4 docs: https://tailwindcss.com/docs

## Version notes

Checked: 2026-09-12 against https://tailwindcss.com/blog and https://tailwindcss.com/docs/theme

- 4.3 (2026-05-08): no directive or function syntax change; no v5 exists.
- 4.0: CSS-first config (`@import "tailwindcss"`, `@theme`, `@utility`, `--alpha()`), automatic content detection, `not-`, `*:`, and `starting:` variants. Theme values are read with `var(--*)`, not a `--theme()` function (corrected 2026-09-12).
- 3.x: `tailwind.config.js`, `@tailwind base/components/utilities`, `theme()` helper. See "v3 patterns to avoid in v4".
