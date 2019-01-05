#!/bin/bash
SCRIPT_NAME="$(basename ${BASH_SOURCE[0]})"
pushd $(dirname ${BASH_SOURCE[0]}) > /dev/null
SCRIPT_DIR=$(pwd)
pushd ../.. > /dev/null
SOURCE_DIR=$(pwd)
popd > /dev/null
pushd ../functions > /dev/null
FN_DIR=$(pwd)
popd > /dev/null
popd > /dev/null

source "${SCRIPT_DIR}/functions.sh"

usage() {
cat <<-EOF
Usage: ${SCRIPT_NAME} [<options ...>]

Description:
   This script will build the Consul binary on the local system.
   All the requisite tooling must be installed for this to be
   successful.

Options:
                       
   -s | --source     DIR         Path to source to build.
                                 Defaults to "${SOURCE_DIR}"
                                 
   -o | --os         OSES        Space separated string of OS
                                 platforms to build.
                                 
   -a | --arch       ARCH        Space separated string of
                                 architectures to build.
   
   -h | --help                   Print this help text.
EOF
}

err_usage() {
  err "$1"
  err ""
  err "$(usage)"
}

main() {
  local _sdir="${SOURCE_DIR}"
  local _build_os=""
  local _build_arch=""
   
  while (( $# )); do
    case "$1" in
      -h | --help )
        usage
        return 0
        ;;
      -s | --source )
        if [[ -z "$2" ]]; then
          err_usage "ERROR: option -s/--source requires an argument"
          return 1
        fi

        if ! [[ -d "$2" ]]; then
          err_usage "ERROR: '$2' is not a directory and not suitable for the value of -s/--source"
          return 1
        fi
 
        _sdir="$2"
        shift 2
        ;;
      -o | --os )
        if [[ -z "$2" ]]; then
          err_usage "ERROR: option -o/--os requires an argument"
          return 1
        fi
           
        _build_os="$2"
        shift 2
        ;;
      -a | --arch )
        if [[ -z "$2" ]]; then
          err_usage "ERROR: option -a/--arch requires an argument"
          return 1
        fi
 
        _build_arch="$2"
        shift 2
        ;;
      * )
        err_usage "ERROR: Unknown argument: '$1'"
        return 1
        ;;
    esac
  done

  build_consul_local "${_sdir}" "${_build_os}" "${_build_arch}" || return 1

  return 0
}

main "$@"
exit $?
