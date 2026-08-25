#!/usr/bin/env bash

set -euo pipefail

readonly ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly REQUIRED=(
  compose.yml
  configuration-file.yml
  configuration-sql.yml
  users_database.yml
  Caddyfile
  diagnostic-app.go
  diagnostic.Dockerfile
  README.md
)

for file in "${REQUIRED[@]}"; do
  if [[ ! -f "${ROOT}/${file}" ]]; then
    echo "Missing rehearsal file: ${file}" >&2
    exit 1
  fi
done

if rg -n '\$\{[^}]+\}' "${ROOT}/compose.yml"; then
  echo "Compose must use literal values without variable substitution" >&2
  exit 1
fi

grep -Fq 'authelia-mtl:rehearsal' "${ROOT}/compose.yml"
grep -Fq 'linux/amd64' "${ROOT}/compose.yml"
grep -Fq 'auth.rehearsal.test' "${ROOT}/Caddyfile"
grep -Fq 'app.rehearsal.test' "${ROOT}/Caddyfile"
grep -Fq 'Remote-Email' "${ROOT}/Caddyfile"
grep -Fq 'generated_email_domain: eurmtl.me' "${ROOT}/configuration-sql.yml"

echo "rehearsal static verification passed"
