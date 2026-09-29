#!/usr/bin/env bash
# Cross-builds kit into ../bin/kit-<os>-<arch>. Reproducible: CI rebuilds with
# the same Go version (go.mod) and fails when a committed binary differs.
# Usage: build.sh            every target
#        build.sh --host     only this machine's target
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

targets=(darwin/arm64 darwin/amd64 linux/arm64 linux/amd64)
if [[ "${1:-}" == --host ]]; then
  targets=("$(go env GOOS)/$(go env GOARCH)")
fi

for target in "${targets[@]}"; do
  out="../bin/kit-${target%/*}-${target#*/}"
  CGO_ENABLED=0 GOOS="${target%/*}" GOARCH="${target#*/}" \
    go build -trimpath -buildvcs=false -ldflags="-s -w -buildid=" -o "$out" ./cmd/kit
  echo "built $out"
done
