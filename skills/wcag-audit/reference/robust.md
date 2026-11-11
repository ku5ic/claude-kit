# 4. Robust

- 4.1.2 Name, Role, Value (A): custom widgets expose the correct role, state, and properties via ARIA where native semantics fall short.
- 4.1.3 Status Messages (AA): live regions announce async status without a focus change.

## ARIA: native semantics first

- First Rule of ARIA Use: when a native HTML element or attribute already has the semantics and behavior you need, use it instead of adding ARIA to another element. `<button>` before `<div role="button">`, `<a href>` before `<div role="link">`, `<input type="checkbox">` before `<div role="checkbox">`.
- Second Rule of ARIA Use: don't change native semantics unless you really have to.
- Use ARIA when no native element has the semantics you need. Combobox, tablist, tree, treegrid, and certain landmark roles are legitimate ARIA territory, each with a canonical pattern in the ARIA Authoring Practices Guide (APG).

## APG patterns

- Dialog (Modal): focus trap, Esc to close, and focus restoration on close.
- Combobox: the ARIA 1.2 pattern with `aria-expanded`, `aria-controls`, and `aria-activedescendant`, for typeahead with suggestions. Native `<select>` covers the basic case.
- Menu and menubar: arrow keys, Home/End, and type-to-select. Use them only for application menus that mimic desktop UI, never for site navigation, which uses `<nav>` and links.
- Tabs: `role="tablist"` with `role="tab"` and `role="tabpanel"`. Arrow keys move between tabs; Tab moves into the active panel.
- A custom widget with no APG pattern: audit it against 4.1.2, where the user agent must report a role, an accessible name, and the current state. Test with VoiceOver (macOS), NVDA (Windows), or the browser's accessibility tree inspector.
