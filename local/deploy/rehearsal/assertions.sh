#!/usr/bin/env bash

assert_contains() {
  local actual="$1"
  local expected="$2"

  if [[ "${actual}" != *"${expected}"* ]]; then
    echo "Expected output to contain: ${expected}" >&2
    echo "Actual output: ${actual}" >&2
    return 1
  fi
}

assert_equals() {
  local expected="$1"
  local actual="$2"
  local description="$3"

  if [[ "${actual}" != "${expected}" ]]; then
    echo "Expected ${description} to be '${expected}', got '${actual}'" >&2
    return 1
  fi
}
