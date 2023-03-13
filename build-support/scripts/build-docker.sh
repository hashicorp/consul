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
Usage: ${SCRIPT_NAME} (consul|ui) [<options ...>]

Description:
   This script will build the various Consul components within docker containers
   and copy all the relevant artifacts out of the containers back to the source.

Options:
   -i | --image      IMAGE       Alternative Docker image to run the build within.
                               
   -s | --source     DIR         Path to source to build.
                                 Defaults to "${SOURCE_DIR}"
                                 
   -r | --refresh                Enables refreshing the docker image prior to building.
   
   -h | --help                   Print this help text.
EOF
}

function err_usage {
   err "$1"
   err ""
   err "$(usage)"
}

function main {
   declare    image=
   declare    sdir="${SOURCE_DIR}"
   declare -i refresh=0
   declare    command=""
   
   while test $# -gt 0
   do
      case "$1" in
         -h | --help )
            usage
            return 0
            ;;
         -i | --image )
            if test -z "$2"
            then
               err_usage "ERROR: option -i/--image requires an argument"
               return 1
            fi
            
            image="$2"
            shift 2
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
         -r | --refresh )
            refresh=1
            shift
            ;;
         consul | ui )
            command="$1"
            shift
            ;;
         * )
            err_usage "ERROR: Unknown argument '$1'"
            return 1
            ;;
      esac
   done
   
   if test -z "${command}"
   then
      err_usage "ERROR: No command specified"
      return 1
   fi
   
   case "${command}" in 
      consul )
         if is_set "${refresh}"
         then
            status_stage "==> Refreshing Consul build container image"
            export GO_BUILD_TAG="${image:-${GO_BUILD_CONTAINER_DEFAULT}}"
            refresh_docker_images "${sdir}" go-build-image || return 1
         fi
         status_stage "==> Building Consul"
         build_consul "${sdir}" "" "${image}" || return 1
         ;;
      ui )
         if is_set "${refresh}"
         then
            status_stage "==> Refreshing UI build container image"
            export UI_BUILD_TAG="${image:-${UI_BUILD_CONTAINER_DEFAULT}}"
            refresh_docker_images "${sdir}" ui-build-image || return 1
         fi
         status_stage "==> Building UI"
         build_ui "${sdir}" "${image}" || return 1
         status "==> UI Built with Version: $(ui_version ${sdir}/agent/uiserver/dist/index.html), Logo: $(ui_logo_type ${sdir}/agent/uiserver/dist/index.html)"
         ;;
      * )
         err_usage "ERROR: Unknown command: '${command}'"
         return 1
         ;;
   esac
   
   return 0
}

main "$@"
exit $?
