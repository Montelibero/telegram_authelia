#!/usr/bin/env bash

set -euo pipefail

readonly ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly COMPOSE=(docker compose -f "${ROOT}/compose.yml")
readonly IMAGE="authelia-mtl:rehearsal"
readonly AUTH_URL="https://auth.rehearsal.test:8443"
readonly APP_URL="https://app.rehearsal.test:8443"
readonly AUTH_RESOLVE="auth.rehearsal.test:8443:127.0.0.1"
readonly APP_RESOLVE="app.rehearsal.test:8443:127.0.0.1"
readonly PROJECT="authelia-mtl-rehearsal"
readonly WORK="$(mktemp -d -t authelia-mtl-rehearsal.XXXXXX)"

# shellcheck source=assertions.sh
source "${ROOT}/assertions.sh"

cleanup_work() {
  if [[ "${WORK}" == /tmp/authelia-mtl-rehearsal.* && -d "${WORK}" ]]; then
    rm -rf -- "${WORK}"
  fi
}
trap cleanup_work EXIT

compose() {
  "${COMPOSE[@]}" "$@"
}

wait_for_portal() {
  local attempt

  for attempt in $(seq 1 60); do
    if curl --insecure --silent --fail --resolve "${AUTH_RESOLVE}" "${AUTH_URL}/api/health" >/dev/null; then
      return 0
    fi
    sleep 1
  done

  compose logs authelia-file authelia-sql caddy >&2 || true
  echo "Authelia portal did not become ready" >&2
  return 1
}

assert_rehearsal_volumes() {
  local volume label

  for volume in "${PROJECT}_rehearsal-data" "${PROJECT}_caddy-data"; do
    if ! docker volume inspect "${volume}" >/dev/null 2>&1; then
      continue
    fi

    label="$(docker volume inspect --format '{{index .Labels "org.montelibero.authelia.rehearsal"}}' "${volume}")"
    assert_equals "true" "${label}" "${volume} rehearsal label"
  done
}

reset_rehearsal() {
  assert_rehearsal_volumes
  compose --profile file --profile sql down --volumes --remove-orphans
}

login() {
  local username="$1"
  local password="$2"
  local cookie_jar="$3"
  local response_file="$4"

  curl \
    --insecure \
    --silent \
    --show-error \
    --output "${response_file}" \
    --write-out '%{http_code}' \
    --cookie-jar "${cookie_jar}" \
    --resolve "${AUTH_RESOLVE}" \
    --header 'Content-Type: application/json' \
    --data "{\"username\":\"${username}\",\"password\":\"${password}\",\"keepMeLoggedIn\":false,\"targetURL\":\"${APP_URL}/\"}" \
    "${AUTH_URL}/api/firstfactor"
}

request_app() {
  local cookie_jar="$1"
  local response_file="$2"

  curl \
    --insecure \
    --silent \
    --show-error \
    --output "${response_file}" \
    --write-out '%{http_code}' \
    --cookie "${cookie_jar}" \
    --resolve "${APP_RESOLVE}" \
    "${APP_URL}/"
}

run_import() {
  compose --profile sql run --rm authelia-sql \
    /app/authelia \
    --config /config/configuration.yml \
    storage user import \
    --from /config/users_database.yml \
    "$@"
}

verify_login_and_headers() {
  local prefix="$1"
  local status body

  status="$(login rehearsal-user 'RehearsalPass2!' "${WORK}/${prefix}-user.cookies" "${WORK}/${prefix}-user-login.json")"
  assert_equals "200" "${status}" "ordinary-user login status"

  status="$(request_app "${WORK}/${prefix}-user.cookies" "${WORK}/${prefix}-user-app.txt")"
  assert_equals "200" "${status}" "ordinary-user application status"
  body="$(<"${WORK}/${prefix}-user-app.txt")"
  assert_contains "${body}" "Remote-User: rehearsal-user"
  assert_contains "${body}" "Remote-Email: rehearsal-user@eurmtl.me"
  assert_contains "${body}" "Remote-Groups: app:diagnostic"
  assert_contains "${body}" "Remote-Name: Rehearsal User"

  status="$(login rehearsal-admin 'RehearsalPass1!' "${WORK}/${prefix}-admin.cookies" "${WORK}/${prefix}-admin-login.json")"
  assert_equals "200" "${status}" "administrator login status"
  status="$(request_app "${WORK}/${prefix}-admin.cookies" "${WORK}/${prefix}-admin-app.txt")"
  assert_equals "200" "${status}" "administrator application status"
  body="$(<"${WORK}/${prefix}-admin-app.txt")"
  assert_contains "${body}" "Remote-Email: admin@example.test"

  status="$(login rehearsal-disabled 'DisabledPass1!' "${WORK}/${prefix}-disabled.cookies" "${WORK}/${prefix}-disabled-login.json")"
  assert_equals "401" "${status}" "disabled-user login status"

  status="$(login rehearsal-denied 'RehearsalPass2!' "${WORK}/${prefix}-denied.cookies" "${WORK}/${prefix}-denied-login.json")"
  assert_equals "200" "${status}" "ACL-denied user login status"
  status="$(request_app "${WORK}/${prefix}-denied.cookies" "${WORK}/${prefix}-denied-app.txt")"
  assert_equals "403" "${status}" "ACL-denied application status"
}

bash "${ROOT}/verify.sh"
bash "${ROOT}/assertions-test.sh"
compose config --quiet

docker run --rm -v "${ROOT}:/rehearsal:ro" "${IMAGE}" \
  /app/authelia validate-config --config /rehearsal/configuration-file.yml >/dev/null
docker run --rm -v "${ROOT}:/rehearsal:ro" "${IMAGE}" \
  /app/authelia validate-config --config /rehearsal/configuration-sql.yml >/dev/null

reset_rehearsal
compose --profile file up --build --detach
wait_for_portal

unauthenticated_status="$(curl --insecure --silent --show-error --output /dev/null --write-out '%{http_code}' --resolve "${APP_RESOLVE}" "${APP_URL}/")"
assert_equals "302" "${unauthenticated_status}" "unauthenticated application status"

dry_run="$(run_import --dry-run)"
assert_contains "${dry_run}" "dry-run=true created=4 unchanged=0 conflicts=0"

first_import="$(run_import)"
assert_contains "${first_import}" "dry-run=false created=4 unchanged=0 conflicts=0"

second_import="$(run_import)"
assert_contains "${second_import}" "dry-run=false created=0 unchanged=4 conflicts=0"

compose --profile file stop authelia-file
compose --profile file rm --force authelia-file
compose --profile sql up --detach authelia-sql
wait_for_portal
verify_login_and_headers initial

compose --profile sql up --detach --force-recreate authelia-sql
wait_for_portal
verify_login_and_headers recreated

compose --profile sql stop authelia-sql
compose --profile sql rm --force authelia-sql
compose --profile file up --detach authelia-file
wait_for_portal

rollback_status="$(login rehearsal-user 'RehearsalPass2!' "${WORK}/rollback.cookies" "${WORK}/rollback-login.json")"
assert_equals "200" "${rollback_status}" "file-backend rollback login status"
rollback_status="$(request_app "${WORK}/rollback.cookies" "${WORK}/rollback-app.txt")"
assert_equals "200" "${rollback_status}" "file-backend rollback application status"
assert_contains "$(<"${WORK}/rollback-app.txt")" "Remote-User: rehearsal-user"

assert_rehearsal_volumes
echo "complete migration, persistence, ACL, and rollback rehearsal passed"
