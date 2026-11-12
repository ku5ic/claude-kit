# Dependencies (all stacks)

- Keep a lockfile in the repo, and Dependabot or an equivalent enabled or acknowledged.
- Flag `npm audit`, `pnpm audit`, `pip-audit`, or `bundler-audit` findings at moderate or above.
- Transitive deps: check `package.json` for unexpected scoped packages and typosquat risk.

## Audit tools: lockfile vs full tree

- Run both: the ecosystem-native audit for fast feedback, and OSV-Scanner for cross-ecosystem coverage.
- `npm audit` and its equivalents check the lockfile's resolved packages against the registry's advisories endpoint. They only flag advisories triaged into the npm advisories feed.
- OSV-Scanner queries the OSV federated database, which aggregates GitHub Security Advisories, RustSec, PyPA, RubyAdvisory, and others. It covers npm, PyPI, Maven, Go modules, and container images, with more findings to triage.

## SLSA basics

SLSA (Supply-chain Levels for Software Artifacts) grades the trustworthiness of build provenance. v1.2 defines Build L0 (no requirements, the absence of SLSA) and three graded build levels:

- Build L1: provenance exists and documents how the package was built. It's trivial to bypass or forge, so it catches mistakes and doesn't defend against an adversary.
- Build L2: a hosted build platform signs the provenance. Forging it requires an explicit attack on the build platform.
- Build L3: hardened builds, with strong tamper protection during the build, isolation between runs, and signing secrets protected from user-defined steps. Forging it requires exploiting a build-platform vulnerability.

- First-party application code: check that the build runs on a trusted platform (GitHub Actions with signed artifacts is L2) and that dependency artifacts carry provenance the build can verify.
- Use SLSA framing in a supply-chain review, or when designing a build pipeline that must defend against compromise.
