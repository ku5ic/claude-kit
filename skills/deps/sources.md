# deps sources

The sources and version notes behind this skill, for `/meta-refresh`. Nothing links here, so a session never loads it.

## reference/bots.md

### References

- Dependabot options, `pull-request-branch-name`: https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference
- Dependabot version updates, the `.github` config file: https://docs.github.com/en/code-security/dependabot/dependabot-version-updates/configuring-dependabot-version-updates
- Dependabot PR comment commands: https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-pull-request-comment-commands
- Dependabot rebase strategy, 30 days: https://docs.github.com/en/code-security/dependabot/working-with-dependabot/managing-pull-requests-for-dependency-updates
- Renovate config file names and order: https://github.com/renovatebot/renovate/blob/main/lib/config/app-strings.ts
- Renovate `branchPrefix` and `rebaseLabel`: https://docs.renovatebot.com/configuration-options/
- Renovate rebasing, and edits stopping it: https://docs.renovatebot.com/updating-rebasing/

### Version notes

Checked: 2026-11-16 against the Dependabot docs and Renovate main.

- Renovate: storing config in `package.json` is deprecated.

## SKILL.md

### References

- Semantic Versioning 2.0.0, for the patch, minor, and major classes: https://semver.org/
