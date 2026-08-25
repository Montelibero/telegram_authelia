#!/usr/bin/env bash

set -euo pipefail

readonly IMAGE="${1:-authelia-mtl:rehearsal}"

architecture="$(docker image inspect --format '{{.Architecture}}' "${IMAGE}")"
if [[ "${architecture}" != "amd64" ]]; then
  echo "Expected amd64 image, got: ${architecture}" >&2
  exit 1
fi

docker run --rm "${IMAGE}" /app/authelia --version
docker run --rm "${IMAGE}" /app/authelia storage user import --help >/dev/null

echo "deployment image smoke test passed"
