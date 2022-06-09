#!/bin/bash
readonly SCRIPT_NAME="$(basename ${BASH_SOURCE[0]})"
readonly SCRIPT_DIR="$(dirname ${BASH_SOURCE[0]})"
readonly SOURCE_DIR="$(dirname "$(dirname "${SCRIPT_DIR}")")"
readonly FN_DIR="$(dirname "${SCRIPT_DIR}")/functions"

source "${SCRIPT_DIR}/functions.sh"

function usage {
cat <<-EOF
Usage: ${SCRIPT_NAME}  [<options ...>]

Description:

   This script uses the date of the last checkin on the branch as the build date. This
   is to make the date consistent across the various platforms we build on, even if they
   start at different times. In practice this is the commit where the version string is set.

Options:
   -s | --source     DIR         Path to source to build.
                                 Defaults to "${SOURCE_DIR}"

EOF
}

function err_usage {
   err "$1"
   err ""
   err "$(usage)"
}

function main {
   declare    sdir="${SOURCE_DIR}"
   declare -i date=0

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
         *)
            err_usage "ERROR: Unknown argument: '$1'"
            return 1
            ;;
      esac
   done

   git_date "${sdir}" || return 1

   return 0
}

main "$@"
exit $?
