#!/bin/bash

set -euo pipefail

# verify_deb.sh tries to install the .deb package at the path given before running `consul version` as a simple smoke test.
# This is meant to be run as part of the build workflow to verify the built .deb meets some basic criteria for validity.
function main {
  local deb_path="${1:-}"

  if [[ -z "${deb_path}" ]]; then
    echo "ERROR: package path argument is required"
    exit 1
  fi

  if [[ ! -e "${deb_path}" ]]; then
    echo "ERROR: package at ${deb_path} does not exist."
    exit 1
  fi

  apt -y update
  apt -y install openssl
  dpkg -i "${deb_path}"

  consul version
}

main "$@"
