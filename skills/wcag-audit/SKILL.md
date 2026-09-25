---
name: wcag-audit
description: WCAG 2.2 AA audit checklist, severity rubric, and stack adaptation for accessibility review. Use whenever the project contains UI code (`.jsx`, `.tsx`, `.vue`, `.svelte`, HTML files, Django templates), OR the user asks about accessibility, a11y, WCAG, screen readers, keyboard navigation, focus management, ARIA, contrast, semantic HTML, alt text, form labels, or audits a component, page, or template for accessibility issues regardless of stack, even if WCAG is not mentioned by name.
---

# WCAG 2.2 AA audit

Target: WCAG 2.2 Level AA. Includes all Level A. Does not include AAA unless explicitly requested.

## How to apply

1. Go through the references in order.
2. Skip criteria that do not apply to the scope (e.g. no forms -> skip form labeling).
3. Record what was checked, what passed, what failed.
4. Note items that require runtime or user testing.

For accessibility work, validate against WCAG 2.2 AA explicitly. Do not claim compliance without checking it against this list; an unaudited claim of conformance is a fabrication.

## Severity rubric

- `failure`: a direct Level AA violation that will block certification or cause a usable defect.
- `warning`: likely to cause issues in some assistive tech configurations, or violates A level and relied on by AA.
- `info`: best practice not required at AA, or a pattern likely to regress if not fixed now.

## Reference files

| File                                                               | Covers                                                                                                     |
| ------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------- |
| [reference/anti-patterns.md](reference/anti-patterns.md)           | Severity-labeled anti-patterns: non-semantic interactives, contrast, missing labels                        |
| [reference/perceivable.md](reference/perceivable.md)               | 1.1, 1.3, 1.4 (alt, semantics, contrast, reflow, text spacing) plus contrast tooling and APCA status       |
| [reference/operable.md](reference/operable.md)                     | 2.1, 2.4, 2.5 (keyboard, focus, target size, drag) plus SPA focus management and 2.2 new criteria examples |
| [reference/understandable.md](reference/understandable.md)         | 3.2, 3.3 (consistent nav, errors, labels) plus 3.3.7 / 3.3.8 cognitive accessibility implementations       |
| [reference/robust.md](reference/robust.md)                         | 4.1.2 / 4.1.3 (name/role/value, status messages) plus First Rule of ARIA and APG patterns                  |
| [reference/cannot-be-verified.md](reference/cannot-be-verified.md) | Items requiring runtime / AT testing, plus React / Django template / plain HTML stack notes                |

## References

- WCAG 2.2 spec: https://www.w3.org/TR/WCAG22/
- Understanding WCAG 2.2: https://www.w3.org/WAI/WCAG22/Understanding/
- How to Meet WCAG 2.2 (Quick Reference): https://www.w3.org/WAI/WCAG22/quickref/
- ARIA Authoring Practices Guide (APG): https://www.w3.org/WAI/ARIA/apg/
- W3C Using ARIA (Notes): https://www.w3.org/TR/using-aria/
- WebAIM: https://webaim.org/
- MDN Accessibility: https://developer.mozilla.org/en-US/docs/Web/Accessibility

## Version notes

WCAG evolves:

- WCAG 3 is in early-draft form at the time of writing and will not replace WCAG 2.2 conformance until after a long sunset period.
- APCA contrast research continues outside the W3C; treat any "use APCA instead of WCAG ratio" claim as out of scope until the W3C ships normative guidance.
- When the W3C publishes a new WCAG version, reconcile this skill against the per-criterion Understanding pages.
