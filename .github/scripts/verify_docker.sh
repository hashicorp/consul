#!/bin/bash

set -euo pipefail

# verify_docker.sh invokes the given Docker image to run `consul version` and inspect its output.
# If its output doesn't match the version given, the script will exit 1 and report why it failed.
# This is meant to be run as part of the build workflow to verify the built image meets some basic
# criteria for validity.

function usage {
  echo "./verify_docker.sh <image_name> <expect_version>"
}

function main {
  local image_name="${1:-}"
  local expect_version="${2:-}"
  local got_version

  if [[ -z "${image_name}" ]]; then
    echo "ERROR: image name argument is required"
    usage
    exit 1
  fi

  if [[ -z "${expect_version}" ]]; then
    echo "ERROR: expected version argument is required"
    usage
    exit 1
  fi

  got_version="$(docker run "${image_name}" version | head -n1 | awk '{print $2}')"
  if [ "${got_version}" != "${expect_version}" ]; then
    echo "Test FAILED"
    echo "Got: ${got_version}, Want: ${expect_version}"
    exit 1
  fi
  echo "Test PASSED"
}

main "$@"
