# Anti-patterns

Severity rubric: [../SKILL.md](../SKILL.md#severity-rubric).

- A `<div>` or `<span>` clicked via JavaScript as an interactive element: `failure`. Use `<button>` for actions and `<a href>` for navigation; if layout forces a non-semantic element, add `role`, `tabindex="0"`, and keyboard event handlers.
- An `<img>` with no `alt` attribute: `failure`. Give an informative image descriptive `alt` text and a decorative one `alt=""`.
- Contrast below 4.5:1 for normal text (under 18pt, or under 14pt bold), or below 3:1 for large text and UI components: `failure`. Measure it with a tool such as browser DevTools, axe, or Colour Contrast Analyser; never estimate it.
- An `aria-label` or `aria-labelledby` that says something different from the visible text label: `failure`. Make the accessible name match or start with the visible text (2.5.3).
- An `<input>`, `<select>`, or `<textarea>` with no associated `<label>` (via `for`/`id` or wrapping), `aria-label`, or `aria-labelledby`: `failure`. Add one; placeholder text doesn't substitute for a label.
- A modal or dialog that lets focus escape, or that has no keyboard or visible close: `failure`. Move focus inside it on open and keep it there until it closes; close it with Escape and with a visible close control.
- Information conveyed by color alone, such as errors shown only in red, required fields marked only by a colored asterisk, or chart series told apart only by hue: `failure` (1.4.1). Pair the color with an icon, label, pattern, or underline.
- `aria-hidden="true"` on a focusable element such as a button, link, or input: `warning`. Remove `aria-hidden`, or make the element non-focusable with `tabindex="-1"` or `disabled`.
