# States and gaps

- Each interactive component's states are marked `shown`, `omitted`, or `n/a`, from default, hover, focus, pressed, disabled, loading, empty, error, success, and selected, plus long text, many items, zero items, and each breakpoint.
- No standard fixes that list. Omitted states go to the designer as questions, not as defects.
- A design system documents states per component (GOV.UK's text input has its error message and hint), never one list for all.
- From the design, check text and non-text contrast, target size, a visible label on each input, errors that don't rely on color alone, the focus order the layout implies, and a name for each icon-only control.
- Static pixels can't show keyboard behavior, roles, reduced motion, or announced updates. List them once as unknown, not per component.
