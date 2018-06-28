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
Usage: ${SCRIPT_NAME}  [<options ...>]

Description:

   This script will verify a Consul release build. It will check for prebuilt
   files, verify shasums and gpg signatures as well as run some commands
   and prompt for manual verification where required.

Options:                       
   -s | --source     DIR         Path to source to build.
                                 Defaults to "${SOURCE_DIR}"
   
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
   declare    vers=""
   
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
         -v | --version )
            if test -z "$2"
            then
               err_usage "ERROR: option -v/--version requires an argument"
               return 1
            fi
            
            vers="$2"
            shift 2
            ;;
         *)
            err_usage "ERROR: Unknown argument: '$1'"
            return 1
            ;;
      esac
   done
   
   if test -z "${vers}"
   then
      vers=$(parse_version "${sdir}" true false)
   fi
   
   status_stage "=> Starting release verification for version: ${version}"
   verify_release_build "${sdir}" "${vers}" || return 1
   
   return 0
}

main "$@"
exit $?
   