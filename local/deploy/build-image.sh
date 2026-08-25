#!/usr/bin/env bash

set -euo pipefail

readonly ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly IMAGE="${1:-authelia-mtl:rehearsal}"
readonly BUILD_COMMIT="$(git -C "${ROOT}" rev-parse HEAD)"

docker build \
  --platform linux/amd64 \
  --file "${ROOT}/local/deploy/Dockerfile" \
  --build-arg "BUILD_COMMIT=${BUILD_COMMIT}" \
  --build-arg BUILD_BRANCH=deploy \
  --build-arg BUILD_TAG=latest \
  --build-arg "BUILD_STATE=untagged clean" \
  --tag "${IMAGE}" \
  "${ROOT}"
