# Dependencies (all stacks)

- Lockfile in repo. Dependabot or equivalent enabled or acknowledged.
- Known CVEs: if `npm audit`, `pnpm audit`, `pip-audit`, or `bundler-audit` show findings at moderate or above, flag them.
- Transitive deps: check `package.json` for unexpected scoped packages, typosquat risk.

## Audit tools: lockfile vs full tree

`npm audit` (and friends) consults the registry's advisories endpoint for the resolved package set in the lockfile. Cheap to run, narrow in scope: only flags advisories the registry knows about. Misses anything that has not been triaged into the npm advisories feed.

OSV-Scanner (https://google.github.io/osv-scanner/) queries the OSV federated database, which aggregates GitHub Security Advisories, RustSec, PyPA, RubyAdvisory, and others. Broader coverage at the cost of more findings to triage; supports multiple ecosystems including npm, PyPI, Maven, Go modules, container images. Run both: ecosystem-native audit for fast feedback, OSV-Scanner for cross-ecosystem coverage.

## SLSA basics

SLSA (Supply-chain Levels for Software Artifacts, https://slsa.dev/spec/v1.0/levels) is a framework for grading the trustworthiness of build provenance. v1.0 defines three build levels:

- **Build L1**: provenance exists. The package documents how it was built. Trivial to bypass or forge; useful for catching mistakes, not for adversarial defense.
- **Build L2**: hosted build platform with signed provenance. Forging the provenance requires an explicit attack on the build platform.
- **Build L3**: hardened builds. Strong tamper protection during the build, isolation between runs, signing secrets protected from user-defined steps. Forgery requires exploiting a build-platform vulnerability.

For first-party application code, the practical question is whether the build runs on a trusted platform (GitHub Actions with signed artifacts is L2) and whether dependency artifacts come with provenance the build can verify. Reach for SLSA framing during a supply-chain review or when designing a build pipeline that needs to defend against compromise.

## References

- OSV-Scanner: https://google.github.io/osv-scanner/
- SLSA spec: https://slsa.dev/spec/v1.0/levels
- npm audit: https://docs.npmjs.com/cli/v10/commands/npm-audit
- pip-audit: https://pypi.org/project/pip-audit/
- bundler-audit: https://github.com/rubysec/bundler-audit
