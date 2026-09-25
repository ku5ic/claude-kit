# 3. Understandable

- 3.2.3 Consistent Navigation (AA): repeated navigation in consistent order.
- 3.2.4 Consistent Identification (AA): components with the same function labeled consistently.
- 3.2.6 Consistent Help (A): help mechanisms in consistent location across pages (new in 2.2).
- 3.3.1 Error Identification (A): errors identified in text.
- 3.3.2 Labels or Instructions (A): controls have labels or instructions.
- 3.3.3 Error Suggestion (AA): suggestions offered when known.
- 3.3.4 Error Prevention (AA): for legal, financial, data submissions: reversible, checkable, or confirmable.
- 3.3.7 Redundant Entry (A): do not ask the same info twice in a process (new in 2.2).
- 3.3.8 Accessible Authentication Minimum (AA): no cognitive function test without alternative (new in 2.2).

## Cognitive accessibility patterns

The two new 2.2 cognitive criteria have concrete implementation paths.

### 3.3.7 Redundant Entry implementations

When a multi-step process collects the same information twice, the second collection must auto-populate or offer a one-action select:

- A "shipping address same as billing" checkbox that, when checked, copies the prior step's values.
- Persisted form state across steps so back-then-forward does not re-prompt for previously entered values.
- For repeat customers, pre-fill from the user's saved profile.

Exceptions per the Understanding doc: re-entering the value is essential (security verification, deliberate re-confirmation), or the previous value is no longer valid (a session timeout that forces re-auth).

### 3.3.8 Accessible Authentication (Minimum) implementations

A "cognitive function test" is something the user must memorize, transcribe, or solve to log in: passwords, image recognition, math puzzles. Per the Understanding doc, sufficient techniques include:

- **Allow password manager autofill**: do not block paste, do not block autofill, expose `<input type="password" autocomplete="current-password">` correctly. The browser's password manager solves the cognitive test for the user.
- **Magic link via email**: the user clicks a link in their inbox; no password to remember.
- **Third-party SSO** (Sign in with Google / Apple / etc.): delegates the cognitive test to a system the user already keeps signed in.
- **Biometric or device-bound auth** (passkeys, WebAuthn, Touch ID): the device proves identity without the user solving anything.

CAPTCHA is a cognitive function test by definition. To meet 3.3.8, the page must offer at least one alternative path that is not a cognitive test (typically: SSO, magic link, or a passkey flow).

## References

- WCAG 2.2 Consistent Help (3.2.6): https://www.w3.org/WAI/WCAG22/Understanding/consistent-help
- WCAG 2.2 Redundant Entry (3.3.7): https://www.w3.org/WAI/WCAG22/Understanding/redundant-entry
- WCAG 2.2 Accessible Authentication Minimum (3.3.8): https://www.w3.org/WAI/WCAG22/Understanding/accessible-authentication-minimum
