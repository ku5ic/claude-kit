# 2. Operable

- 2.4.11 Focus Not Obscured (Minimum) (AA), new in 2.2: a focused control stays at least partially visible, never fully covered by a sticky header, footer, or toast. Account for sticky elements in the scroll position on focus, or move them out of the focus path.
- 2.5.7 Dragging Movements (AA), new in 2.2: every drag interaction has a single-pointer alternative. For drag-and-drop reordering, add arrow-key reorder ("Press Up/Down to move this item") plus an explicit move button.
- 2.5.8 Target Size (Minimum) (AA), new in 2.2: interactive targets are at least 24x24 CSS px or have adequate spacing; inline links and controls in a sentence are exempt. Pad a smaller icon-only button, such as a 16 px toolbar icon, until its hit area meets the threshold.
- 2.1.1 Keyboard (A): every interactive element is reachable and operable via keyboard.
- 2.1.2 No Keyboard Trap (A): focus never gets stuck.
- 2.4.3 Focus Order (A): tab order follows the visual and logical order.
- 2.4.4 Link Purpose in Context (A): link text describes the destination or purpose.
- 2.4.6 Headings and Labels (AA): headings and labels describe their topic.
- 2.4.7 Focus Visible (AA): every interactive element has a visible focus indicator.
- 2.5.3 Label in Name (A): the accessible name contains the visible label.

## Focus management for SPAs

- A route change: move focus to the new view's main heading, or to an `<h1>` or `<main>` region with `tabindex="-1"`.
- A modal opening: move focus to its first focusable element, typically the close button or the first form field, and trap Tab inside the modal.
- A modal closing: restore focus to the element that opened it.
- The APG dialog pattern documents the focus-trap algorithm and every keyboard interaction: Esc closes, Tab and Shift+Tab cycle, and focus returns on close.
