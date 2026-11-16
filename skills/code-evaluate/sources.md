# code-evaluate sources

The sources and version notes behind this skill, for `/meta-refresh`. Nothing links here, so a session never loads it.

## reference/assessment.md

### References

- OpenSSF Concise Guide for Evaluating Open Source Software: https://github.com/ossf/wg-best-practices-os-developers/blob/main/docs/Concise-Guide-for-Evaluating-Open-Source-Software.md
- OpenSSF Scorecard checks and risk levels: https://github.com/ossf/scorecard/blob/main/docs/checks.md
- Semantic Versioning 2.0.0, item 4: https://semver.org/
- SPDX license expressions, Annex D: https://spdx.github.io/spdx-spec/v2.3.1/SPDX-license-expressions/
- SPDX license list: https://spdx.org/licenses/

### Version notes

Checked: 2026-11-16 against the Concise Guide (revised 2025-04-23), Scorecard `docs/checks.md` on main, and the SPDX license list 3.29.0.

## reference/data.md

### References

- deps.dev API v3: https://docs.deps.dev/api/v3/
- OSV query API: https://google.github.io/osv.dev/post-v1-query/
- OSV schema, range events: https://ossf.github.io/osv-schema/
- Scorecard API base: https://github.com/ossf/scorecard/blob/main/README.md

### Version notes

Checked: 2026-11-16 by live calls: deps.dev v3 for `npm/left-pad`, OSV for `lodash@4.17.15`, Scorecard for `github.com/ossf/scorecard`.

- deps.dev documents no dependents count; the report doesn't claim one.
