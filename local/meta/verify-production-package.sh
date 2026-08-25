#!/usr/bin/env bash

set -euo pipefail

readonly ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly COMPOSE="${ROOT}/server-compose-example.yml"
readonly CHECKLIST="${ROOT}/production-cutover-checklist.md"

for file in "${COMPOSE}" "${CHECKLIST}"; do
  if [[ ! -f "${file}" ]]; then
    echo "Missing production handoff file: ${file}" >&2
    exit 1
  fi
done

if rg -n '\$\{[^}]+\}' "${COMPOSE}"; then
  echo "Server Compose must not use variable substitution" >&2
  exit 1
fi

if grep -Fq '.env' "${COMPOSE}"; then
  echo "Server Compose must not depend on an env file" >&2
  exit 1
fi

grep -Fq 'ghcr.io/montelibero/authelia:latest' "${COMPOSE}"
grep -Fq -- '- AUTHELIA_SESSION_SECRET=change_me' "${COMPOSE}"
grep -Fq -- '- AUTHELIA_STORAGE_ENCRYPTION_KEY=change_me' "${COMPOSE}"
grep -Fq 'docker compose pull authelia' "${CHECKLIST}"
grep -Fq 'storage user import' "${CHECKLIST}"
grep -Fq -- '--dry-run' "${CHECKLIST}"
grep -Fq 'Remote-Email' "${CHECKLIST}"
grep -Fq 'Rollback' "${CHECKLIST}"
grep -Fq 'STOP' "${CHECKLIST}"

echo "production handoff static verification passed"
