#!/usr/bin/env bash

set -euo pipefail

readonly ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly IMAGE="${1:-authelia-mtl:rehearsal}"

docker build \
  --platform linux/amd64 \
  --file "${ROOT}/local/deploy/Dockerfile" \
  --tag "${IMAGE}" \
  "${ROOT}"
