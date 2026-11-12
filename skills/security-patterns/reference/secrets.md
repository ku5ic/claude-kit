# Secrets

- A hardcoded token, key, or password in the codebase: `failure`.
- A committed `.env`: `failure`.
- Secrets in commit history: note them. Rewriting history is a destructive operation (`skills/kit-rules/reference/workflow.md` section 2).

## Detection in CI

- Run gitleaks, a regex-plus-entropy scanner for git repos and arbitrary input, as a pre-commit hook and as a CI step on every PR. Use `gitleaks git --no-banner --redact -v` (or `gitleaks dir` for files) for readable output that doesn't echo the secret into the log. `detect` and `protect` are deprecated since v8.19.0.
- trufflehog is a credential-aware scanner that also tries live verification against the credential's issuer, where applicable. It's slower, with fewer false positives.
- Enable GitHub secret-scanning push protection at the repo or org level. It blocks any push containing a secret that matches GitHub's detector list.

## Rotation, not redaction

A secret in commit history that anyone outside the trust boundary could see is compromised. Rotation is the only remediation, in this order:

1. Rotate the credential at the issuer: revoke the old, issue the new.
2. Update every consumer to the new value: deployment configs, CI secrets, team members' local `.env`.
3. Optionally rewrite history with `git filter-repo` to remove the value from future clones. It's cosmetic; never depend on it.
4. Add a pre-commit gitleaks hook to prevent recurrence.

## Rotation cadence

Long-lived secrets that can't be made short-lived (database passwords, encryption keys, third-party API keys without short-lived tokens):

- Rotate on personnel changes (departure, role change).
- Rotate after any suspected exposure (lost laptop, accidental log dump).
- Rotate on a calendar: every 90 days is a common default for non-customer-facing secrets, and shorter for high-stakes systems.

Credentials that support short-lived tokens (cloud IAM, OIDC federation): use those over long-lived keys.
