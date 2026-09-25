# 4. Robust

- 4.1.2 Name, Role, Value (A): custom widgets expose correct role, state, properties via ARIA where native semantics insufficient.
- 4.1.3 Status Messages (AA): live regions for async status without focus change.

## ARIA: prefer native semantics first

Per the W3C "Using ARIA" Note, the **First Rule of ARIA Use** is:

> "If you can use a native HTML element or attribute with the semantics and behavior you require already built in, instead of re-purposing an element and adding an ARIA role, state or property to make it accessible, then do so."

The **Second Rule of ARIA Use**:

> "Do not change native semantics, unless you really have to."

In practice: reach for `<button>` before `<div role="button">`, `<a href>` before `<div role="link">`, `<input type="checkbox">` before `<div role="checkbox">`. Native elements bring keyboard handling, focus behavior, and screen-reader announcements for free; the ARIA-on-div equivalent must reproduce all of that explicitly and rarely does.

ARIA earns its place when there is no native element with the desired semantics. Combobox, tablist, tree, treegrid, and certain landmark roles ARE legitimate ARIA territory because HTML has no equivalent. The ARIA Authoring Practices Guide (APG) documents the canonical pattern for each.

## APG patterns worth bookmarking

- **Dialog (Modal)**: focus trap, Esc to close, focus restoration on close. https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/
- **Combobox**: ARIA 1.2 pattern with `aria-expanded`, `aria-controls`, `aria-activedescendant`. Native `<select>` covers the basic case; combobox is for typeahead-with-suggestions.
- **Menu and menubar**: keyboard navigation with arrow keys, Home/End, type-to-select. Most app menus do not need this; it is for application menus mimicking desktop UI, not for site navigation (which uses `<nav>` and links).
- **Tabs**: `role="tablist"` with `role="tab"` and `role="tabpanel"`. Arrow keys move between tabs; Tab moves into the active panel.

For a custom widget that has no APG pattern, audit against 4.1.2: the user-agent must report a role, an accessible name, and the current state. Test with VoiceOver (macOS), NVDA (Windows), or the browser's accessibility tree inspector.

## References

- WCAG 2.2 Name, Role, Value (4.1.2): https://www.w3.org/WAI/WCAG22/Understanding/name-role-value
- WCAG 2.2 Status Messages (4.1.3): https://www.w3.org/WAI/WCAG22/Understanding/status-messages
- W3C Using ARIA: https://www.w3.org/TR/using-aria/
- ARIA Authoring Practices Guide: https://www.w3.org/WAI/ARIA/apg/
