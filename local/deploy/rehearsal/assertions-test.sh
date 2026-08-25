#!/usr/bin/env bash

set -euo pipefail

readonly ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=assertions.sh
source "${ROOT}/assertions.sh"

assert_contains "created=4 unchanged=0 conflicts=0" "created=4"
assert_equals "amd64" "amd64" "architecture"

if (assert_equals "expected" "actual" "negative self-test") 2>/dev/null; then
  echo "assert_equals accepted a mismatch" >&2
  exit 1
fi

echo "rehearsal assertion self-test passed"
