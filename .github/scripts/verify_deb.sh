#!/bin/bash

set -euo pipefail

# verify_deb.sh tries to install the .deb package at the path given before running `consul version`
# to inspect its output. If its output doesn't match the version given, the script will exit 1 and
# report why it failed. This is meant to be run as part of the build workflow to verify the built
# .deb meets some basic criteria for validity.

# set this so we can locate and execute the verify_bin.sh script for verifying version output
SCRIPT_DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

function usage {
  echo "./verify_deb.sh <path_to_deb> <expect_version>"
}

function main {
  local deb_path="${1:-}"
  local expect_version="${2:-}"
  local got_version

  if [[ -z "${deb_path}" ]]; then
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
  deb_path=$(echo ${deb_path})

  if [[ ! -e "${deb_path}" ]]; then
    echo "ERROR: package at ${deb_path} does not exist."
    usage
    exit 1
  fi

  # we have to install the 'arm' architecture in order to install the 'arm'
  # package, otherwise we will git a 'package architecture does not match system' error
  if [[ ${deb_path} = *_arm.deb ]]; then
    dpkg --add-architecture arm
  fi

  apt -y update
  apt -y install openssl
  dpkg -i ${deb_path}

  # use the script that should be located next to this one for verifying the output
  exec "${SCRIPT_DIR}/verify_bin.sh" $(which consul) "${expect_version}"
}

main "$@"
