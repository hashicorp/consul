#!/bin/bash

set -euo pipefail

# verify_rpm.sh tries to install the .rpm package at the path given before running `consul version`
# to inspect its output. If its output doesn't match the version given, the script will exit 1 and
# report why it failed. This is meant to be run as part of the build workflow to verify the built
# .rpm meets some basic criteria for validity.

# set this so we can locate and execute the verify_bin.sh script for verifying version output
SCRIPT_DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

function usage {
  echo "./verify_rpm.sh <path_to_rpm> <expect_version>"
}

function main {
  local rpm_path="${1:-}"
  local expect_version="${2:-}"
  local got_version

  if [[ -z "${rpm_path}" ]]; then
    echo "ERROR: package path argument is required"
    usage
    exit 1
  fi

  if [[ -z "${expect_version}" ]]; then
    echo "ERROR: expected version argument is required"
    usage
    exit 1
  fi

  # expand globs for path names, if this fails, the script will exit
  rpm_path=$(echo ${rpm_path})

  if [[ ! -e "${rpm_path}" ]]; then
    echo "ERROR: package at ${rpm_path} does not exist."
    usage
    exit 1
  fi

  yum -y clean all
  yum -y update
  yum -y install which openssl
  rpm --ignorearch -i ${rpm_path}

  # use the script that should be located next to this one for verifying the output
  exec "${SCRIPT_DIR}/verify_bin.sh" $(which consul) "${expect_version}"
}

main "$@"
