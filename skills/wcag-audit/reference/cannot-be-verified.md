# Cannot be verified statically + stack adaptation

## Cannot be verified statically

Note these in the "Cannot be verified statically" section of the report:

- Runtime focus behavior (modals, route changes, async content)
- Color contrast when colors come from CSS variables with runtime theming
- Screen reader announcement quality (requires AT testing)
- Motion sensitivity (`prefers-reduced-motion` paths need runtime verification)
- Keyboard-only operation flows (need actual keyboard testing)
- Cognitive load of authentication flows

## Stack adaptation

- **React / Next.js**: check ARIA patterns on custom components, effect based focus management, `sr-only` usage, Next.js `Link` composition.
- **Django templates**: check template inheritance patterns for consistent nav, form field rendering via `{{ form.field }}`, crispy-forms or equivalent output.
- **Plain HTML**: no framework quirks, but watch for missing `<main>`, heading hierarchy across included partials.
