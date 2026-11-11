# tailwind-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- v4 release post: https://tailwindcss.com/blog/tailwindcss-v4
- v3-to-v4 upgrade guide: https://tailwindcss.com/docs/upgrade-guide
- v4 docs: https://tailwindcss.com/docs

### Version notes

Checked: 2026-11-10 against https://www.npmjs.com/package/tailwindcss, https://tailwindcss.com/docs/upgrade-guide, https://tailwindcss.com/blog/tailwindcss-v4-3 and https://tailwindcss.com/blog/tailwindcss-v4

- 4.3 (2026-05-08, latest 4.3.5 on 2026-11-06): stacked and compound variants in `@variant`; no v5 exists. Corrected the v4 variant stacking order, the `@config` path for a JS config, and the `@layer utilities` wording.
- 4.0: CSS-first config (`@import "tailwindcss"`, `@theme`, `@utility`, `--alpha()`), automatic content detection, `not-`, `nth-*`, and `starting:` variants (`*:` is 3.4; build-speed figures and the `*:` example corrected 2026-09-26). Theme values are read with `var(--*)`, not a `--theme()` function (corrected 2026-09-12).
- 3.x: `tailwind.config.js`, `@tailwind base/components/utilities`, `theme()` helper. See "v4 setup".

### Left to linters

Checked: 2026-11-11. No default check in the kit's set (ESLint `js.configs.recommended`, `tsc --strict`) reads Tailwind classes or CSS, so no rule was dropped. The v4 build-speed figures were cut as a fact with no rule attached.
