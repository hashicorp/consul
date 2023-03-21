#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


readonly SCRIPT_NAME="$(basename ${BASH_SOURCE[0]})"
readonly SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
readonly SOURCE_DIR="$(dirname "$(dirname "${SCRIPT_DIR}")")"
readonly FN_DIR="$(dirname "${SCRIPT_DIR}")/functions"

source "${SCRIPT_DIR}/functions.sh"

function usage {
cat <<-EOF
Usage: ${SCRIPT_NAME}  [<options ...>]

Description:
   
   This script will do a full release build of Consul. Building each component
   is done within a docker container. In addition to building Consul this
   script will do a few more things.
   
      * Update version/version*.go files
      * Update CHANGELOG.md to put things into release mode
      * Create a release commit. It changes in the commit include the CHANGELOG.md
        version files.
      * Tag the release
      * Generate the SHA256SUMS file for the binaries
      * Sign the SHA256SUMS file with a GPG key


Options:                       
   -s | --source     DIR         Path to source to build.
                                 Defaults to "${SOURCE_DIR}"
                                 
   -t | --tag        BOOL        Whether to add a release commit and tag the build. 
                                 This also controls whether we put the tree into
                                 release mode
                                 Defaults to 1.
                                 
   -b | --build      BOOL        Whether to perform the build of the ui's and
                                 binaries. Defaults to 1.
                                 
   -S | --sign       BOOL        Whether to sign the generated SHA256SUMS file.
                                 Defaults to 1.
                                             
   -g | --gpg-key    KEY         Alternative GPG key to use for signing operations.
                                 Defaults to ${HASHICORP_GPG_KEY}

   -v | --version    VERSION     The version of Consul to be built. If not specified
                                 the version will be parsed from the source.
   
   -d | --date       DATE        The release date. Defaults to today.
   
   -r | --release    STRING      The prerelease version. Defaults to an empty pre-release.
                                 
   -h | --help                   Print this help text.
EOF
}

function err_usage {
   err "$1"
   err ""
   err "$(usage)"
}

function ensure_arg {
   if test -z "$2"
   then
      err_usage "ERROR: option $1 requires an argument"
      return 1
   fi
   
   return 0
}

function main {
   declare    sdir="${SOURCE_DIR}"
   declare -i do_tag=1
   declare -i do_build=1
   declare -i do_sign=1
   declare    gpg_key="${HASHICORP_GPG_KEY}"
   declare    version=""
   declare    release_ver=""
   declare    release_date=$(date +"%B %d, %Y")
   
   while test $# -gt 0
   do
      case "$1" in
         -h | --help )
            usage
            return 0
            ;;
         -s | --source )
            ensure_arg "-s/--source" "$2" || return 1
           
            if ! test -d "$2"
            then
               err_usage "ERROR: '$2' is not a directory and not suitable for the value of -s/--source"
               return 1
            fi
            
            sdir="$2"
            shift 2
            ;;
         -t | --tag )
            ensure_arg "-t/--tag" "$2" || return 1
            do_tag="$2"
            shift 2
            ;;
         -b | --build )
            ensure_arg "-b/--build" "$2" || return 1
            do_build="$2"
            shift 2
            ;;
         -S | --sign )
            ensure_arg "-s/--sign" "$2" || return 1
            do_sign="$2"
            shift 2
            ;;
         -g | --gpg-key )
            ensure_arg "-g/--gpg-key" "$2" || return 1
            gpg_key="$2"
            shift 2
            ;;
         -v | --version )
            ensure_arg "-v/--version" "$2" || return 1
            version="$2"
            shift 2
            ;;
         -d | --date)
            ensure_arg "-d/--date" "$2" || return 1
            release_date="$2"
            shift 2
            ;;
         -r | --release)
            ensure_arg "-r/--release" "$2" || return 1
            release_ver="$2"
            shift 2
            ;;
         *)
            err_usage "ERROR: Unknown argument: '$1'"
            return 1
            ;;
      esac
   done
   
   build_release "${sdir}" "${do_tag}" "${do_build}" "${do_sign}" "${version}" "${release_date}" "${release_ver}" "${gpg_key}"
   return $?
}

main "$@"
exit $?
   
