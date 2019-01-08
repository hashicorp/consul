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
Usage: ${SCRIPT_NAME}  [<options ...>]

Description:

   This script will "publish" a Consul release. It expects a prebuilt release in 
   pkg/dist matching the version in the repo and a clean git status. It will 
   prompt you to confirm the consul version and git changes you are going to 
   publish prior to pushing to git and to releases.hashicorp.com.

Options:                       
   -s | --source     DIR         Path to source to build.
                                 Defaults to "${SOURCE_DIR}"
                                 
   -w | --website                Publish to releases.hashicorp.com
   
   -g | --git                    Push release commit and tag to Git
   
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
   local _website=0
   local _git_push=0
   
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
        -w | --website )
            _website=1
            shift
            ;;
         -g | --git )
            _git_push=1
            shift
            ;;
         *)
            err_usage "ERROR: Unknown argument: '$1'"
            return 1
            ;;
      esac
   done
   
   publish_release "${_sdir}" "${_git_push}" "${_website}" || return 1
   
   return 0
}

main "$@"
exit $?
