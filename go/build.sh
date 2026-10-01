#!/usr/bin/env bash
# Cross-builds kit into ../bin/kit-<version>-<os>-<arch>, the names bin/kit
# looks for and the release workflow uploads. The version is
# .claude-plugin/plugin.json's, stamped into `kit version`. Reproducible:
# same Go version (go.mod), no VCS or build IDs.
# Usage: build.sh            every target
#        build.sh --host     only this machine's target
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

version="$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' ../.claude-plugin/plugin.json | head -1)"
if [[ -z "$version" ]]; then
  echo "build.sh: no version in ../.claude-plugin/plugin.json" >&2
  exit 1
fi

targets=(darwin/arm64 darwin/amd64 linux/arm64 linux/amd64)
if [[ "${1:-}" == --host ]]; then
  targets=("$(go env GOOS)/$(go env GOARCH)")
fi

for target in "${targets[@]}"; do
  out="../bin/kit-$version-${target%/*}-${target#*/}"
  CGO_ENABLED=0 GOOS="${target%/*}" GOARCH="${target#*/}" \
    go build -trimpath -buildvcs=false -ldflags="-s -w -buildid= -X main.version=$version" -o "$out" ./cmd/kit
  echo "built $out"
done
