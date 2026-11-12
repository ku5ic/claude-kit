# 1. Perceivable

- 1.1.1 Non-text Content (A): images have alt text or are marked decorative; icon-only buttons have an accessible name.
- 1.3.1 Info and Relationships (A): semantic HTML over ARIA, a correct heading hierarchy, lists in `ul`/`ol`/`li`, tables with `th`/`caption`, and form fields with a `label`.
- 1.3.4 Orientation (AA): no forced orientation lock.
- 1.3.5 Identify Input Purpose (AA): personal data fields carry autocomplete tokens.
- 1.4.3 Contrast Minimum (AA): 4.5:1 for body text, 3:1 for large text. Flag hardcoded colors that may fail; note where runtime theming prevents static verification.
- 1.4.10 Reflow (AA): content works at 320 CSS px without horizontal scroll.
- 1.4.11 Non-text Contrast (AA): UI component and graphical object boundaries reach 3:1.
- 1.4.12 Text Spacing (AA): content survives line-height 1.5 and letter spacing 0.12em.
- 1.4.13 Content on Hover or Focus (AA): it's dismissible, hoverable, and persistent.

## Color contrast tooling and APCA

- WCAG 2.x contrast is the luminance ratio (L1 + 0.05) / (L2 + 0.05).
- The Chrome and Firefox DevTools color picker shows the WCAG 2 ratio for an inspected color and flags AA and AAA pass or fail.
- The axe DevTools browser extension runs the checks across a page and lists every failing element.
- The Stark and Contrast design plugins catch contrast issues during design, before code.
- APCA (Advanced Perceptual Contrast Algorithm) is a contrast model from outside the W3C process. It's not part of WCAG 2.2, not normative in any published WCAG version, and not named in the WCAG 3 introduction; treat it as research literature.
- When a stakeholder cites APCA scores, ask whether the certification target is WCAG 2.2, where the L1/L2 ratio counts, or another policy.
