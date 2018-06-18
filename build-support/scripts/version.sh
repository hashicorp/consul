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

   This script is just a convenience around discover what the Consul
   version would be if you were to build it. 

Options:                       
   -s | --source     DIR         Path to source to build.
                                 Defaults to "${SOURCE_DIR}"
                                 
   -r | --release                Include the release in the version
   
   -g | --git                    Take git variables into account
   
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
   declare -i release=0
   declare -i git_info=0
   
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
         -r | --release )
            release=1
            shift
            ;;
         -g | --git )
            git_info=1
            shift
            ;;
         *)
            err_usage "ERROR: Unknown argument: '$1'"
            return 1
            ;;
      esac
   done
   
   parse_version "${sdir}" "${release}" "${git_info}" || return 1
   
   return 0
}

main "$@"
exit $?
   