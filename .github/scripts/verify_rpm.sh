#!/bin/bash

set -euo pipefail

# verify_rpm.sh tries to install the RPM at the path given before running `consul version` as a simple smoke test.
# This is meant to be run as part of the build workflow to verify the built RPM meets some basic criteria for validity.
function main {
  local rpm_path="${1:-}"

  if [[ -z "${rpm_path}" ]]; then
    echo "ERROR: RPM path argument is required"
    exit 1
  fi

  if [[ ! -e "${rpm_path}" ]]; then
    echo "ERROR: RPM at ${rpm_path} does not exist."
    exit 1
  fi

  yum -y update
  yum -y install openssl
  rpm -i "${rpm_path}"

  consul version
}

main "$@"
