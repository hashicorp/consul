#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -euo pipefail

# verify_rpm.sh tries to install the .rpm package at the path given before running `consul version`
# to inspect its output. If its output doesn't match the version given, the script will exit 1 and
# report why it failed. This is meant to be run as part of the build workflow to verify the built
# .rpm meets some basic criteria for validity.

# Notably, CentOS 7 is EOL, so we need to point to the vault for updates. It's not clear what alternative
# we may use in the future that supports linux/386 as the platform was dropped in CentOS 8+9. The docker_image
# is passed in as the third argument so that the script can determine if it needs to point to the vault for updates.

# set this so we can locate and execute the verify_bin.sh script for verifying version output
SCRIPT_DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

function usage {
  echo "./verify_rpm.sh <path_to_rpm> <expect_version>"
}

function main {
  local rpm_path="${1:-}"
  local expect_version="${2:-}"
  local docker_image="${3:-}"
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

  if [[ -z "${docker_image}" ]]; then
    echo "ERROR: docker image argument is required"
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

  # CentOS 7 is EOL, so we need to point to the vault for updates
  if [[ "$docker_image" == *centos:7 ]]; then
    sed -i 's/mirrorlist/#mirrorlist/g' /etc/yum.repos.d/CentOS-*
    sed -i 's|#baseurl=http://mirror.centos.org|baseurl=http://vault.centos.org|g' /etc/yum.repos.d/CentOS-*
  fi

  yum -y clean all
  yum -y update
  yum -y install which openssl
  rpm --ignorearch -i ${rpm_path}

  # use the script that should be located next to this one for verifying the output
  exec "${SCRIPT_DIR}/verify_bin.sh" $(which consul) "${expect_version}"
}

main "$@"
