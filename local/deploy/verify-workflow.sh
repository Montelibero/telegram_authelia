#!/usr/bin/env bash

set -euo pipefail

readonly ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly WORKFLOW="${ROOT}/.github/workflows/deploy-image.yml"
readonly DOCKERFILE="${ROOT}/local/deploy/Dockerfile"
readonly DOCKERIGNORE="${DOCKERFILE}.dockerignore"

if [[ ! -f "${WORKFLOW}" ]]; then
  echo "Missing deployment image workflow: ${WORKFLOW}" >&2
  exit 1
fi

require_literal() {
  local value="$1"
  if ! grep -Fq -- "${value}" "${WORKFLOW}"; then
    echo "Workflow is missing required value: ${value}" >&2
    exit 1
  fi
}

require_literal "branches: [deploy]"
require_literal "workflow_dispatch:"
require_literal "packages: write"
require_literal "contents: read"
require_literal "platforms: linux/amd64"
require_literal "ghcr.io/montelibero/authelia:latest"
require_literal "push: true"
require_literal "file: ./local/deploy/Dockerfile"

if [[ ! -f "${DOCKERFILE}" ]]; then
  echo "Missing deployment Dockerfile: ${DOCKERFILE}" >&2
  exit 1
fi

if [[ ! -f "${DOCKERIGNORE}" ]]; then
  echo "Missing deployment Docker ignore file: ${DOCKERIGNORE}" >&2
  exit 1
fi

if grep -Eq 'uses: [^#[:space:]]+@(v[0-9]+|main|master)([[:space:]]|$)' "${WORKFLOW}"; then
  echo "Workflow contains a mutable action reference" >&2
  exit 1
fi

while IFS= read -r reference; do
  if [[ ! "${reference}" =~ @[0-9a-f]{40}$ ]]; then
    echo "Action reference is not pinned to a full commit SHA: ${reference}" >&2
    exit 1
  fi
done < <(sed -nE 's/^[[:space:]]*-[[:space:]]uses:[[:space:]]*([^#[:space:]]+).*/\1/p' "${WORKFLOW}")

if grep -Eq '(^|[[:space:]])git[[:space:]]+(push|tag)|gh[[:space:]]+release' "${WORKFLOW}"; then
  echo "Workflow must not mutate branches or tags" >&2
  exit 1
fi

echo "deploy-image workflow verification passed"
