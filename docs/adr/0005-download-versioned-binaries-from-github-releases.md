# 5. Download versioned binaries from GitHub releases

Date: 2026-10-01

## Status

Accepted

## Context

The binary from [4](0004-run-every-hook-from-one-go-binary.md) has to match the plugin version the user installed, on four platforms. Committing binaries to the repo grows it with every release.

## Decision

`bin/kit` runs `kit-<plugin.json version>-<os>-<arch>`, for darwin and linux on arm64 and amd64. When the binary is missing, it downloads it from the GitHub release `v<version>` (commit 674bc06). With `KIT_DEV=1` it builds from `go/` instead. Binaries are never committed. Only the CLI and the session-start hooks download, since any other hook would be killed mid-download (0.2.1).

## Consequences

- A release has to publish the binaries before the plugin version that names them goes out.
- Until the first download lands, every guard is off, per [6](0006-fail-open-when-the-kit-itself-breaks.md). Offline, a read-only plugin directory, a slow link, or another platform leaves it missing.
- In this repo, `bin/kit` with `KIT_DEV=1` tests the checkout's code. A bare `kit` on PATH is the installed release.
