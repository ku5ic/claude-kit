# Secrets

- Any hardcoded token, key, or password in the codebase: `failure`.
- `.env` committed: `failure`.
- Secrets in commit history: note, do not attempt to fix without asking.

## Detection in CI

Two complementary tools cover detection at different points in the workflow:

- **gitleaks** (https://github.com/gitleaks/gitleaks): regex-plus-entropy scanner for git repos and arbitrary input. Run as a pre-commit hook for fast local feedback, and as a CI step on every PR. Useful flags: `gitleaks detect --no-banner --redact -v` for human-readable output that does not echo the secret value into the log.
- **trufflehog** (https://github.com/trufflesecurity/trufflehog): credential-aware scanner that goes one step further by attempting live verification against the credential's issuer (where applicable). Slower but produces fewer false positives.

GitHub also provides server-side scanning:

- **Secret-scanning push protection**: enabled at the repo or org level. Blocks any push that contains a secret pattern matching GitHub's detector list. Catches what slips past the local hook. Documented at <https://docs.github.com/en/code-security/secret-scanning/>.

## Rotation, not redaction

A secret that landed in commit history is compromised the moment the commit was visible to anyone outside the trust boundary. Rotation is the only remediation; rewriting history does not retroactively secure forks, mirrors, or local clones that any reader may have made.

The standard remediation sequence:

1. Rotate the credential at the issuer (revoke the old, issue the new).
2. Update every consumer of the credential to the new value (deployment configs, CI secrets, team members' local `.env`).
3. Optionally rewrite history with `git filter-repo` to remove the value from future clones, but treat this as cosmetic; do not depend on it.
4. Add a pre-commit gitleaks hook to prevent recurrence.

## Rotation cadence

For long-lived secrets that cannot be made short-lived (database passwords, encryption keys, third-party API keys without short-lived tokens):

- Rotate on personnel changes (departure, role change).
- Rotate after any suspected exposure event (lost laptop, accidental log dump).
- Rotate on a calendar (every 90 days is a common default for non-customer-facing secrets; high-stakes systems shorter).

For credentials that support short-lived tokens (cloud IAM, OIDC federation), prefer those over long-lived keys: the rotation problem becomes the issuer's instead of yours.

## References

- gitleaks: https://github.com/gitleaks/gitleaks
- trufflehog: https://github.com/trufflesecurity/trufflehog
- GitHub Secret Scanning: https://docs.github.com/en/code-security/secret-scanning/
