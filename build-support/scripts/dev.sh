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

function usage {
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

function err_usage {
   err "$1"
   err ""
   err "$(usage)"
}

function main {
   declare    sdir="${SOURCE_DIR}"
   declare    build_os=""
   declare    build_arch=""
   declare -i do_git=1
   declare -i do_push=1
   
   
   while test $# -gt 0
   do
      case "$1" in
         -h | --help )
            usage
            return 0
            ;;
         -s | --source )
            if test -z "$2"
            then
               err_usage "ERROR: option -s/--source requires an argument"
               return 1
            fi
            
            if ! test -d "$2"
            then
               err_usage "ERROR: '$2' is not a directory and not suitable for the value of -s/--source"
               return 1
            fi
            
            sdir="$2"
            shift 2
            ;;
         --no-git )
            do_git=0
            shift
            ;;
         --no-push )
            do_push=0
            shift
            ;;
         * )
            err_usage "ERROR: Unknown argument: '$1'"
            return 1
            ;;
      esac
   done
   
   set_dev_mode "${sdir}" || return 1
   
   if is_set "${do_git}"
   then
      status_stage "==> Commiting Dev Mode Changes"
      commit_dev_mode "${sdir}" || return 1
      
      if is_set "${do_push}"
      then
         status_stage "==> Confirming Git Changes"
         confirm_git_push_changes "${sdir}" || return 1
         
         status_stage "==> Pushing to Git"
         git_push_ref "${sdir}" || return 1
      fi
   fi
   
   return 0
}

main "$@"
exit $?