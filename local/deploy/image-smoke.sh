#!/usr/bin/env bash

set -euo pipefail

readonly IMAGE="${1:-authelia-mtl:rehearsal}"
readonly EXPECTED_COMMIT="${2:-$(git -C "$(dirname "${BASH_SOURCE[0]}")/../.." rev-parse HEAD)}"

architecture="$(docker image inspect --format '{{.Architecture}}' "${IMAGE}")"
if [[ "${architecture}" != "amd64" ]]; then
  echo "Expected amd64 image, got: ${architecture}" >&2
  exit 1
fi

revision="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "${IMAGE}")"
if [[ "${revision}" != "${EXPECTED_COMMIT}" ]]; then
  echo "Expected image revision ${EXPECTED_COMMIT}, got: ${revision}" >&2
  exit 1
fi

version="$(docker run --rm "${IMAGE}" /app/authelia --version)"
if [[ "${version}" != *"${EXPECTED_COMMIT:0:7}"* ]]; then
  echo "Image version does not identify revision ${EXPECTED_COMMIT}: ${version}" >&2
  exit 1
fi
docker run --rm "${IMAGE}" /app/authelia storage user import --help >/dev/null

echo "deployment image smoke test passed"
