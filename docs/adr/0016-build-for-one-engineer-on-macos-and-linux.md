# 16. Build for one engineer on macOS and Linux

Date: 2026-11-11

## Status

Accepted

## Context

A team policy layer and Windows support each widen what every feature has to handle, and the kit has one author.

## Decision

The kit is for one engineer on macOS or Linux. There's no team policy layer and no Windows support. Decided by the author for 0.10.0.

## Consequences

- Config is one user's overlay on `kit.yml`, with no organization layer above it.
- Binaries ship for darwin and linux only ([5](0005-download-versioned-binaries-from-github-releases.md)).
- A request for either one is answered with this record, until a new ADR supersedes it.
