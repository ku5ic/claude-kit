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

Read every file in this table as soon as this skill loads. A hook blocks every tool call until you have.

| Before                                                                                                    | Read                                                               |
| --------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------ |
| Rating a finding's severity, or reviewing for common accessibility anti-patterns                          | [reference/anti-patterns.md](reference/anti-patterns.md)           |
| Checking alt text, semantics, color contrast, reflow, or text spacing                                     | [reference/perceivable.md](reference/perceivable.md)               |
| Checking keyboard access, focus, target size, or SPA focus management                                     | [reference/operable.md](reference/operable.md)                     |
| Checking consistent navigation, error handling, labels, redundant entry, or authentication                | [reference/understandable.md](reference/understandable.md)         |
| Checking names, roles, values, status messages, or ARIA and APG patterns                                  | [reference/robust.md](reference/robust.md)                         |
| Listing what static review cannot verify, or adapting the audit to React, Django templates, or plain HTML | [reference/cannot-be-verified.md](reference/cannot-be-verified.md) |
