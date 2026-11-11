# wcag-audit sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- WCAG 2.2 spec: https://www.w3.org/TR/WCAG22/
- Understanding WCAG 2.2: https://www.w3.org/WAI/WCAG22/Understanding/
- How to Meet WCAG 2.2 (Quick Reference): https://www.w3.org/WAI/WCAG22/quickref/
- ARIA Authoring Practices Guide (APG): https://www.w3.org/WAI/ARIA/apg/
- W3C Using ARIA (Discontinued Draft since 2026-02-24, kept for its rules; current guidance is the APG): https://www.w3.org/TR/using-aria/
- WebAIM: https://webaim.org/
- MDN Accessibility: https://developer.mozilla.org/en-US/docs/Web/Accessibility

### Version notes

Checked: 2026-11-10 against https://www.w3.org/TR/WCAG22/, https://www.w3.org/TR/wcag-3.0/, https://www.w3.org/TR/wai-aria-1.3/ and https://www.w3.org/TR/using-aria/

- WCAG 2.2 (Recommendation, updated 2024-12-12): current target; 4.1.1 Parsing obsolete; WAI-ARIA 1.2 still the current Recommendation (1.3 is a Working Draft, 2026-06-04). WCAG 3.0 remains a Working Draft (2026-09-10). Using ARIA became a Discontinued Draft on 2026-02-24.

WCAG evolves:

- WCAG 3 is in early-draft form at the time of writing and will not replace WCAG 2.2 conformance until after a long sunset period.
- APCA contrast research continues outside the W3C; treat any "use APCA instead of WCAG ratio" claim as out of scope until the W3C ships normative guidance.
- When the W3C publishes a new WCAG version, reconcile this skill against the per-criterion Understanding pages.

### Left to linters

Checked: 2026-11-11. No default check in the kit's set (ESLint `js.configs.recommended`, `tsc --strict`, shellcheck with no flags, ruff defaults) reads accessibility semantics: ARIA, `alt`, labels, contrast, focus, or target size. No rule was dropped, and nothing was probed. axe, eslint-plugin-jsx-a11y, Lighthouse, and pa11y aren't in that set and don't count.

`reference/robust.md` no longer names the source of the First and Second Rules of ARIA; that source and its Discontinued Draft status are in the SKILL.md References above.

## reference/anti-patterns.md

### References

- WCAG 2.2 spec: https://www.w3.org/TR/WCAG22/
- First Rule of ARIA: https://www.w3.org/TR/using-aria/#rule1
- ARIA Authoring Practices Guide: https://www.w3.org/WAI/ARIA/apg/
- WCAG 1.4.3 Contrast (Minimum): https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html
- WCAG 2.5.3 Label in Name: https://www.w3.org/WAI/WCAG22/Understanding/label-in-name.html
- WebAIM Contrast Checker: https://webaim.org/resources/contrastchecker/

## reference/operable.md

### References

- WCAG 2.2 Focus Not Obscured (Minimum): https://www.w3.org/WAI/WCAG22/Understanding/focus-not-obscured-minimum
- WCAG 2.2 Dragging Movements: https://www.w3.org/WAI/WCAG22/Understanding/dragging-movements
- WCAG 2.2 Target Size Minimum: https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum
- ARIA APG dialog (modal) pattern: https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/

## reference/perceivable.md

### References

- WCAG 2.2 Contrast Minimum (1.4.3): https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum
- WCAG 2.2 Non-text Contrast (1.4.11): https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast
- axe DevTools: https://www.deque.com/axe/devtools/

### Claims

- APCA, a contrast model from outside the W3C process: https://www.myndex.com/APCA/

## reference/robust.md

### References

- WCAG 2.2 Name, Role, Value (4.1.2): https://www.w3.org/WAI/WCAG22/Understanding/name-role-value
- WCAG 2.2 Status Messages (4.1.3): https://www.w3.org/WAI/WCAG22/Understanding/status-messages
- W3C Using ARIA: https://www.w3.org/TR/using-aria/
- ARIA Authoring Practices Guide: https://www.w3.org/WAI/ARIA/apg/

### Claims

- APG patterns, Dialog (Modal): focus trap, Esc to close, focus restoration on close: https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/

## reference/understandable.md

### References

- WCAG 2.2 Consistent Help (3.2.6): https://www.w3.org/WAI/WCAG22/Understanding/consistent-help
- WCAG 2.2 Redundant Entry (3.3.7): https://www.w3.org/WAI/WCAG22/Understanding/redundant-entry
- WCAG 2.2 Accessible Authentication Minimum (3.3.8): https://www.w3.org/WAI/WCAG22/Understanding/accessible-authentication-minimum
