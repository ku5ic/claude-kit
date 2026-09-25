# 1. Perceivable

- 1.1.1 Non-text Content (A): images have alt or are marked decorative; icon only buttons have an accessible name.
- 1.3.1 Info and Relationships (A): semantic HTML over ARIA, correct heading hierarchy, lists use `ul`/`ol`/`li`, tables use `th`/`caption`, form fields have `label`.
- 1.3.4 Orientation (AA): no forced orientation lock.
- 1.3.5 Identify Input Purpose (AA): autocomplete tokens on personal data fields.
- 1.4.3 Contrast Minimum (AA): 4.5:1 body, 3:1 large text. Flag hardcoded colors that may fail; note where runtime theming prevents static verification.
- 1.4.10 Reflow (AA): content works at 320 CSS px without horizontal scroll.
- 1.4.11 Non-text Contrast (AA): UI component and graphical object boundaries 3:1.
- 1.4.12 Text Spacing (AA): content survives line-height 1.5, letter spacing 0.12em.
- 1.4.13 Content on Hover or Focus (AA): dismissible, hoverable, persistent.

## Color contrast tooling and APCA

WCAG 2.x measures contrast as a luminance ratio (L1 + 0.05) / (L2 + 0.05). Practical tooling:

- Chrome / Firefox DevTools color picker shows the WCAG 2 ratio when inspecting a color value; the picker also flags AA / AAA pass / fail.
- axe DevTools (browser extension): runs the checks across a page and lists every failing element.
- Stark and Contrast (design-side plugins): catch contrast issues during design before they reach code.

APCA (Advanced Perceptual Contrast Algorithm, https://www.myndex.com/APCA/) is an alternative contrast model authored outside the W3C process. It is NOT part of WCAG 2.2 and is NOT currently a normative algorithm in any published WCAG version. The WCAG 3 introduction does not name APCA at the time of writing. Treat it as research literature, not as a normative replacement: if a stakeholder cites APCA scores, ask whether the certification target is WCAG 2.2 (in which case the L1/L2 ratio is what counts) or some other policy.

## References

- WCAG 2.2 Contrast Minimum (1.4.3): https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum
- WCAG 2.2 Non-text Contrast (1.4.11): https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast
- axe DevTools: https://www.deque.com/axe/devtools/
