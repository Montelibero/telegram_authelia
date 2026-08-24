#!/usr/bin/env bash

set -euo pipefail

readonly BASE_BRANCH="release-base"
readonly DEPLOY_BRANCH="deploy"
readonly OVERLAYS=(
  "local/meta"
  "local/auth-overlay"
)

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Refusing to rebuild deploy with a dirty worktree." >&2
  exit 1
fi

if ! git show-ref --verify --quiet "refs/heads/${BASE_BRANCH}"; then
  echo "Missing local stable baseline branch: ${BASE_BRANCH}" >&2
  exit 1
fi

for branch in "${OVERLAYS[@]}"; do
  if ! git show-ref --verify --quiet "refs/heads/${branch}"; then
    echo "Missing local overlay branch: ${branch}" >&2
    exit 1
  fi
done

if git show-ref --verify --quiet "refs/heads/${DEPLOY_BRANCH}"; then
  git switch "${DEPLOY_BRANCH}"
else
  git switch --create "${DEPLOY_BRANCH}" "${BASE_BRANCH}"
fi

git reset --hard "${BASE_BRANCH}"

for branch in "${OVERLAYS[@]}"; do
  git merge --no-ff "${branch}" -m "deploy: include ${branch}"
done

echo "Deploy branch rebuilt locally. Review and test it before any push or deployment."
