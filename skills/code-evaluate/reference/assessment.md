# Assessment

What to check before adopting a package, in order, adapted from the OpenSSF guide for evaluating open source software. A check with no evidence either way is reported as unknown, never as passed.

## Necessity and authenticity

- Necessity: can the repo, the standard library, or an installed dependency do it? Each added dependency is code to keep current and a surface to attack.
- Authenticity: is it the real project, not a fork or impostor? Compare the registry entry's repository link with the project's own site and docs.
- Name: is there a more popular package with a similar name? A near-miss name is a typosquatting risk.

## Maintenance

- Activity: commits or releases in the last 12 months.
- Maintainers: more than one, ideally from more than one organization. A single maintainer is a bus-factor risk, named in the report.
- Version stability: a `0.y.z` version promises no stable API under Semantic Versioning, and anything may change; a pre-release tag says the same.
- Deprecation: the registry's deprecated flag, and a README or repo marked archived.

## Security

- Known vulnerabilities in the version to adopt, and whether past ones were fixed promptly, older release lines included.
- Scorecard: the overall score and the high-risk checks (Code-Review, Maintained, Branch-Protection, Token-Permissions, Signed-Releases, Dependency-Update-Tool, Vulnerabilities, Binary-Artifacts) and the critical ones (Dangerous-Workflow, Webhooks). Each low score is a finding with the check's reason.
- A security policy with a way to report vulnerabilities.
- Install-time code, and obfuscated or minified code in a source package, get read before adopting.

## Adoption and license

- License: an SPDX identifier or expression, checked against the project's own license and use. In an expression, `OR` is a choice, `AND` means all apply, and `WITH` attaches an exception. No license, or a custom one, is a `failure` until someone with authority signs off.
- Adoption: how widely it's used, as one signal, never a substitute for the checks above.

## Footprint

- Direct and transitive dependencies it adds, from its resolved graph; packages the repo already has don't count.
- Dev or test dependencies that leak into the runtime graph.

## Interface

- Stability: a documented breaking-change policy, or a changelog that marks breaking changes.
- Secure defaults, and docs on using it safely, for a package that handles input, auth, crypto, or network.
