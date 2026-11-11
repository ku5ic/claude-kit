# 3. Understandable

- 3.2.6 Consistent Help (A), new in 2.2: help mechanisms sit in a consistent location across pages.
- 3.3.7 Redundant Entry (A), new in 2.2: a process never asks for the same information twice.
- 3.3.8 Accessible Authentication (Minimum) (AA), new in 2.2: no cognitive function test without an alternative.
- 3.2.3 Consistent Navigation (AA): repeated navigation keeps a consistent order.
- 3.2.4 Consistent Identification (AA): components with the same function are labeled consistently.
- 3.3.1 Error Identification (A): errors are identified in text.
- 3.3.2 Labels or Instructions (A): controls have labels or instructions.
- 3.3.3 Error Suggestion (AA): suggestions are offered when known.
- 3.3.4 Error Prevention (AA): legal, financial, and data submissions are reversible, checkable, or confirmable.

## 3.3.7 Redundant Entry implementations

- When a multi-step process collects the same information twice, auto-populate it the second time or offer a one-action select.
- A "shipping address same as billing" checkbox copies the prior step's values.
- Persist form state across steps, so going back and then forward doesn't ask for entered values again.
- For a repeat customer, pre-fill from their saved profile.
- Exceptions, per the Understanding doc: re-entering the value is essential (security verification, deliberate re-confirmation), or the previous value is no longer valid (a session timeout that forces re-auth).

## 3.3.8 Accessible Authentication (Minimum) implementations

A cognitive function test is anything the user must memorize, transcribe, or solve to log in, such as a password, image recognition, or a math puzzle. Sufficient techniques, per the Understanding doc:

- Password manager autofill: never block paste or autofill, and expose `<input type="password" autocomplete="current-password">` correctly.
- A magic link via email.
- Third-party SSO, such as Sign in with Google or Apple.
- Biometric or device-bound auth: passkeys, WebAuthn, Touch ID.
- A CAPTCHA is a cognitive function test. To meet 3.3.8, a page with one also offers at least one path without a cognitive test, typically SSO, a magic link, or a passkey flow.
