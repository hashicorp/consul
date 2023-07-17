#!/bin/bash

set -euo pipefail

# verify_bin.sh validates the file at the path given and then runs `./consul version` and inspects its output. If its
# output doesn't match the version given, the script will exit 1 and report why it failed.
# This is meant to be run as part of the build workflow to verify the built .zip meets some basic criteria for validity.

function usage {
  echo "./verify_bin.sh <path_to_bin> <expect_version>"
}

function main {
  local bin_path="${1:-}"
  local expect_version="${2:-}"
  local got_version

  if [[ -z "${bin_path}" ]]; then
    echo "ERROR: path to binary argument is required"
    usage
    exit 1
  fi

  if [[ -z "${expect_version}" ]]; then
    echo "ERROR: expected version argument is required"
    usage
    exit 1
  fi

  if [[ ! -e "${bin_path}" ]]; then
    echo "ERROR: package at ${bin_path} does not exist."
    exit 1
  fi
  
  got_version="$( awk '{print $2}' <(head -n1 <(${bin_path} version)) )"
  if [ "${got_version}" != "${expect_version}" ]; then
    echo "Test FAILED"
    echo "Got: ${got_version}, Want: ${expect_version}"
    exit 1
  fi
  echo "Test PASSED"
}

main "$@"
