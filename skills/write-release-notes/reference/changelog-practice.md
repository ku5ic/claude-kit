# Release notes practice

## Audience

- Notes are for the people who use the project, not its authors.
- A commit log is raw material, never the notes. Merge commits, unclear titles, and doc-only changes are noise. One entry may sum up several commits.

## Types

- Added: new features.
- Changed: changes in existing functionality.
- Deprecated: features to be removed soon.
- Removed: features now removed.
- Fixed: bug fixes.
- Security: vulnerabilities fixed.

## Never missed

- Deprecations, removals, and breaking changes, so a reader can upgrade to the release that deprecates a thing, drop its use, then upgrade past its removal.

## Release header

- Newest release first, one heading per release, dated in ISO 8601 (2026-11-16).
- An Unreleased section on top holds pending changes; they move under the version at release time.
- A release pulled for a serious bug stays listed, marked `[YANKED]`.
- Say whether versions follow Semantic Versioning.
