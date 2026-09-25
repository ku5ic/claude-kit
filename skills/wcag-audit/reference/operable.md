# 2. Operable

- 2.1.1 Keyboard (A): all interactive elements reachable and operable via keyboard.
- 2.1.2 No Keyboard Trap (A): focus does not get stuck.
- 2.4.3 Focus Order (A): tab order follows visual and logical order.
- 2.4.4 Link Purpose in Context (A): link text describes destination or purpose.
- 2.4.6 Headings and Labels (AA): headings and labels describe topic.
- 2.4.7 Focus Visible (AA): visible focus indicator on all interactive elements.
- 2.4.11 Focus Not Obscured Minimum (AA): focused element not fully hidden by sticky headers, toasts, etc. (new in 2.2).
- 2.5.3 Label in Name (A): accessible name contains the visible label.
- 2.5.7 Dragging Movements (AA): drag interactions have a single pointer alternative (new in 2.2).
- 2.5.8 Target Size Minimum (AA): interactive targets at least 24x24 CSS px or have adequate spacing (new in 2.2).

## Focus management for SPAs

Single-page apps that swap views without a full page load break the browser's default focus restoration. Three patterns from the ARIA APG keep focus behavior accessible:

- **Route changes**: when a route transitions, move focus to the new view's main heading (or an `<h1>` / `<main>` region with `tabindex="-1"`). Without this, screen-reader users continue reading the previous content while sighted users see the new view.
- **Modal open**: when a modal opens, move focus to the first focusable element inside (typically the close button or the first form field). Trap focus inside the modal so Tab cycles through its interactive elements without escaping back to the obscured page underneath.
- **Modal close**: restore focus to the trigger element (the button that opened the modal). Otherwise focus jumps back to the document body and the user has to find their place again.

The APG dialog pattern documents the focus-trap algorithm and lists every keyboard interaction (Esc closes, Tab/Shift+Tab cycles, focus restoration on close).

## Examples for the new 2.2 criteria

- **2.4.11 Focus Not Obscured (Minimum)**: the focused control must be at least partially visible. A sticky footer that completely covers the focused button at the bottom of the page violates this. Fix: account for sticky elements when calculating scroll position on focus, or move the sticky element out of the focus path.
- **2.5.7 Dragging Movements**: drag-and-drop reordering must offer a single-pointer alternative. Common implementation: arrow-key reorder ("Press Up/Down to move this item") plus an explicit move button.
- **2.5.8 Target Size (Minimum)**: 24x24 CSS pixels minimum. Inline links exempt; controls in a sentence exempt. Common violation: 16-pixel icon-only buttons in a toolbar with no padding. Fix: add padding so the hit area meets the threshold even if the visible icon is smaller.

## References

- WCAG 2.2 Focus Not Obscured (Minimum): https://www.w3.org/WAI/WCAG22/Understanding/focus-not-obscured-minimum
- WCAG 2.2 Dragging Movements: https://www.w3.org/WAI/WCAG22/Understanding/dragging-movements
- WCAG 2.2 Target Size Minimum: https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum
- ARIA APG dialog (modal) pattern: https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/
