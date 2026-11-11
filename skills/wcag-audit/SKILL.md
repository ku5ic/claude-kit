---
name: wcag-audit
description: WCAG 2.2 AA audit checklist and severity rubric. Use for accessibility, a11y, ARIA, keyboard, focus, contrast, or form-label questions, or an accessibility audit, in any stack.
---

# WCAG 2.2 AA audit

Target: WCAG 2.2 Level AA, which includes all of Level A. Audit AAA only when asked.

## How to apply

1. Go through the references in order.
2. Skip a criterion the scope doesn't touch, such as form labeling when there are no forms.
3. Record what was checked, what passed, and what failed.
4. Note each item that needs runtime or user testing. Where the project runs axe, it settles some against a running page, such as contrast (1.4.3); screen reader and keyboard behavior still need a person.

Validate accessibility work against WCAG 2.2 AA explicitly. Never claim conformance without checking it against this list; an unaudited claim is a fabrication.

## Severity rubric

- `failure`: a direct Level AA violation that blocks certification or causes a usability defect.
- `warning`: likely to cause issues in some assistive tech configurations, or a Level A violation that AA relies on.
- `info`: a best practice AA doesn't require, or a pattern likely to regress if not fixed now.

## Reference files

| File                                                               | Covers                                                                                                     |
| ------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------- |
| [reference/anti-patterns.md](reference/anti-patterns.md)           | Eight anti-patterns with severity calls: non-semantic interactives, contrast, missing labels               |
| [reference/perceivable.md](reference/perceivable.md)               | 1.1, 1.3, 1.4 (alt, semantics, contrast, reflow, text spacing) plus contrast tooling and APCA status       |
| [reference/operable.md](reference/operable.md)                     | 2.2's 2.4.11, 2.5.7, 2.5.8 with fixes, then 2.1, 2.4, 2.5.3 (keyboard, focus) and SPA focus management     |
| [reference/understandable.md](reference/understandable.md)         | 2.2's 3.2.6, 3.3.7, 3.3.8, then 3.2, 3.3 (consistent nav, errors, labels), plus 3.3.7 and 3.3.8 techniques |
| [reference/robust.md](reference/robust.md)                         | 4.1.2 / 4.1.3 (name/role/value, status messages) plus the First and Second Rules of ARIA and APG patterns  |
| [reference/cannot-be-verified.md](reference/cannot-be-verified.md) | Items requiring runtime / AT testing, plus React / Django template / plain HTML stack notes                |
