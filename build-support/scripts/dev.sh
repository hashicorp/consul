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

   This script will put the source back into dev mode after a release.

Options:
                       
   -s | --source     DIR         Path to source to build.
                                 Defaults to "${SOURCE_DIR}"
                                 
   --no-git                      Do not commit or attempt to push
                                 the changes back to the upstream.
                                 
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
   local _do_git=1
   local _do_push=1
   
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
         --no-git )
            _do_git=0
            shift
            ;;
         --no-push )
            _do_push=0
            shift
            ;;
         * )
            err_usage "ERROR: Unknown argument: '$1'"
            return 1
            ;;
      esac
   done
   
   set_dev_mode "${_sdir}" || return 1
   
   if [[ $(is_set "${_do_git}") == 0 ]]; then
      status_stage "==> Commiting Dev Mode Changes"
      commit_dev_mode "${_sdir}" || return 1
      
      if [[ $(is_set "${_do_push}") == 0 ]]; then
         status_stage "==> Confirming Git Changes"
         confirm_git_push_changes "${_sdir}" || return 1
         
         status_stage "==> Pushing to Git"
         git_push_ref "${_sdir}" || return 1
      fi
   fi
   
   return 0
}

main "$@"
exit $?
