# Cannot be verified statically + stack adaptation

## Cannot be verified statically

List these in the report's "Cannot be verified statically" section:

- Runtime focus behavior: modals, route changes, async content.
- Color contrast when colors come from CSS variables with runtime theming.
- Screen reader announcement quality, which needs AT testing.
- Motion sensitivity: `prefers-reduced-motion` paths need runtime verification.
- Keyboard-only operation flows, which need real keyboard testing.
- The cognitive load of authentication flows.

## Stack adaptation

- React / Next.js: check ARIA patterns on custom components, effect-based focus management, `sr-only` usage, and Next.js `Link` composition.
- Django templates: check template inheritance for consistent nav, form fields rendered via `{{ form.field }}`, and crispy-forms or equivalent output.
- Plain HTML: no framework quirks, but check for a missing `<main>` and for the heading hierarchy across included partials.
