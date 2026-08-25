#!/usr/bin/env bash

set -euo pipefail

readonly SCRIPT_SOURCE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/rebuild-deploy.sh"
readonly TEST_ROOT="$(mktemp -d)"

cleanup() {
  rm -rf "${TEST_ROOT}"
}
trap cleanup EXIT

git -C "${TEST_ROOT}" init --quiet
git -C "${TEST_ROOT}" config user.name "Deploy Test"
git -C "${TEST_ROOT}" config user.email "deploy-test@example.com"

touch "${TEST_ROOT}/base"
git -C "${TEST_ROOT}" add base
git -C "${TEST_ROOT}" commit --quiet -m "base"
git -C "${TEST_ROOT}" branch release-base

for branch in local/meta local/auth-overlay local/ci-deploy; do
  git -C "${TEST_ROOT}" switch --quiet --create "${branch}" release-base
  marker="${branch//\//-}"
  touch "${TEST_ROOT}/${marker}"
  git -C "${TEST_ROOT}" add "${marker}"
  git -C "${TEST_ROOT}" commit --quiet -m "${branch}"
done

git -C "${TEST_ROOT}" switch --quiet release-base
cp "${SCRIPT_SOURCE}" "${TEST_ROOT}/rebuild-deploy.sh"
git -C "${TEST_ROOT}" add rebuild-deploy.sh
git -C "${TEST_ROOT}" commit --quiet -m "test harness"
(
  cd "${TEST_ROOT}"
  bash ./rebuild-deploy.sh >/dev/null
)

test -f "${TEST_ROOT}/local-meta"
test -f "${TEST_ROOT}/local-auth-overlay"
test -f "${TEST_ROOT}/local-ci-deploy"
test "$(git -C "${TEST_ROOT}" branch --show-current)" = "deploy"

touch "${TEST_ROOT}/dirty"
if (
  cd "${TEST_ROOT}"
  bash ./rebuild-deploy.sh >/dev/null 2>&1
); then
  echo "rebuild-deploy.sh accepted a dirty worktree" >&2
  exit 1
fi

echo "rebuild-deploy.sh tests passed"
