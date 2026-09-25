# Anti-patterns

Severity rubric:

- `failure`: a direct Level AA violation that will block certification or cause a usable defect.
- `warning`: likely to cause issues in some assistive tech configurations, or violates A level and relied on by AA.
- `info`: best practice not required at AA, or a pattern likely to regress if not fixed now.

## `<div>` or `<span>` as an interactive element

`failure`. A non-semantic element clicked via JavaScript has no role, no keyboard access, and no accessible name. Screen reader users and keyboard-only users cannot reach it. Use `<button>` for actions and `<a href>` for navigation. If the element must be non-semantic for layout reasons, add `role`, `tabindex="0"`, and keyboard event handlers.

## `<img>` with missing `alt` attribute

`failure`. An image with no `alt` attribute is announced with its filename by most screen readers, which is meaningless. Informative images need a descriptive `alt` text. Decorative images that carry no information need `alt=""` so screen readers skip them.

## Insufficient color contrast

`failure`. WCAG 2.2 AA requires a contrast ratio of at least 4.5:1 for normal text (under 18pt / 14pt bold) and 3:1 for large text and UI components. Failing this criterion makes text unreadable for users with low vision. Measure with a tool (browser DevTools, axe, Colour Contrast Analyser) rather than estimating.

## `aria-label` text that differs from visible label text

`failure`. When an element has a visible text label and an `aria-label` or `aria-labelledby` that says something different, speech input users (Dragon NaturallySpeaking) cannot activate the element by speaking what they see. The accessible name must match or start with the visible text (WCAG 2.5.3).

## Form input without a programmatic label

`failure`. An `<input>`, `<select>`, or `<textarea>` with no associated `<label>` (via `for`/`id` or wrapping), `aria-label`, or `aria-labelledby` has no accessible name. Screen readers announce only "edit" with no context. Placeholder text does not substitute for a label -- it disappears on entry.

## Modal without a focus trap and a close mechanism

`failure`. When a modal or dialog opens, focus must move inside it and be constrained there until it closes. Focus that escapes to the background content disorients screen reader users. The modal must also be closeable via keyboard (Escape key) and have a visible close mechanism.

## Conveying information through color alone

`failure`. Error states shown only in red, required fields indicated only by a color asterisk, and chart series differentiated only by hue all fail WCAG 1.4.1. Pair color with an icon, label, pattern, or underline so the information survives in greyscale and for color-blind users.

## `aria-hidden="true"` on a focusable element

`warning`. A focusable element (button, link, input) with `aria-hidden="true"` is hidden from the accessibility tree but still reachable by keyboard. A screen reader user tabs into it and hears nothing. Either remove `aria-hidden` or make the element non-focusable (`tabindex="-1"`, `disabled`).

## References

- WCAG 2.2 spec: https://www.w3.org/TR/WCAG22/
- First Rule of ARIA: https://www.w3.org/TR/using-aria/#rule1
- ARIA Authoring Practices Guide: https://www.w3.org/WAI/ARIA/apg/
- WCAG 1.4.3 Contrast (Minimum): https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html
- WCAG 2.5.3 Label in Name: https://www.w3.org/WAI/WCAG22/Understanding/label-in-name.html
- WebAIM Contrast Checker: https://webaim.org/resources/contrastchecker/
