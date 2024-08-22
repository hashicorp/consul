#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


readonly SCRIPT_NAME="$(basename ${BASH_SOURCE[0]})"
readonly SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
readonly SOURCE_DIR="$(dirname "$(dirname "${SCRIPT_DIR}")")"
readonly FN_DIR="$(dirname "${SCRIPT_DIR}")/functions"

source "${SCRIPT_DIR}/functions.sh"

function usage {
cat <<-EOF
Usage: ${SCRIPT_NAME}  [<options ...>]

Description:

   This script reports the consul module versions in each of the go.mod files in the Consul repository.

Options:                       
   -h | --help                   Print this help text.
EOF
}

function err_usage {
   err "$1"
   err ""
   err "$(usage)"
}

function main {
   while test $# -gt 0
   do
      case "$1" in
         -h | --help )
            usage
            return 0
            ;;
         *)
            err_usage "ERROR: Unknown argument: '$1'"
            return 1
            ;;
      esac
   done
   
   get_consul_module_versions || return 1
   
   return 0
}

main "$@"
exit $?
   
