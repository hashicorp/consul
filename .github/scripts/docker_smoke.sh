#!/bin/bash

set -euo pipefail

function main {
  local image_name="${1:-${IMAGE_NAME:-}}"
  local test_version

  if [[ -z "${image_name}" ]]; then
    echo "ERROR: IMAGE_NAME environment var or argument is required"
    exit 1
  fi

  test_version="$(docker run "${image_name}" version  | head -n1 | awk '{print $2}')"
  if [ "${test_version}" != "v${version}" ]; then
    echo "Test FAILED"
    echo "Got: ${test_version}, Want: ${version}"
    exit 1
  fi
  echo "Test PASSED"
}

main "$@"
