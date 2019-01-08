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
Usage: ${SCRIPT_NAME} (consul|ui|ui-legacy|static-assets) [<options ...>]

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

err_usage() {
   err "$1"
   err ""
   err "$(usage)"
}

main() {
   local _image=
   local _sdir="${SOURCE_DIR}"
   local _refresh=0
   local _command=""
   
   while (( $# )); do
      case "$1" in
         -h | --help )
            usage
            return 0
            ;;
         -i | --image )
            if [[ -z "$2" ]]; then
               err_usage "ERROR: option -i/--image requires an argument"
               return 1
            fi
            
            _image="$2"
            shift 2
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
         -r | --refresh )
            _refresh=1
            shift
            ;;
         consul | ui | ui-legacy | static-assets )
            _command="$1"
            shift
            ;;
         * )
            err_usage "ERROR: Unknown argument '$1'"
            return 1
            ;;
      esac
   done
   
   if [[ -z "${_command}" ]]; then
      err_usage "ERROR: No command specified"
      return 1
   fi
   
   case "${_command}" in 
      consul )
        if [[ $(is_set "${_refresh}") == 0 ]]; then
            status_stage "==> Refreshing Consul build container image"
            export GO_BUILD_TAG="${_image:-${GO_BUILD_CONTAINER_DEFAULT}}"
            refresh_docker_images "${_sdir}" go-build-image || return 1
         fi
         status_stage "==> Building Consul"
         build_consul "${_sdir}" "" "${_image}" || return 1
         ;;
      static-assets )
        if [[ $(is_set "${_refresh}") == 0 ]]; then
            status_stage "==> Refreshing Consul build container image"
            export GO_BUILD_TAG="${_image:-${GO_BUILD_CONTAINER_DEFAULT}}"
            refresh_docker_images "${_sdir}" go-build-image || return 1
         fi
         status_stage "==> Building Static Assets"
         build_assetfs "${_sdir}" "${_image}" || return 1
         ;;
      ui )
        if [[ $(is_set "${_refresh}") == 0 ]]; then
            status_stage "==> Refreshing UI build container image"
            export UI_BUILD_TAG="${_image:-${UI_BUILD_CONTAINER_DEFAULT}}"
            refresh_docker_images "${_sdir}" ui-build-image || return 1
         fi
         status_stage "==> Building UI"
         build_ui "${_sdir}" "${_image}" || return 1
         status "==> UI Built with Version: $(ui_version ${_sdir}/pkg/web_ui/v2/index.html), Logo: $(ui_logo_type ${_sdir}/pkg/web_ui/v2/index.html)"
         ;;
      ui-legacy )
        if [[ $(is_set "${_refresh}") == 0 ]]; then
            status_stage "==> Refreshing Legacy UI build container image"
            export UI_LEGACY_BUILD_TAG="${_image:-${UI_LEGACY_BUILD_CONTAINER_DEFAULT}}"
            refresh_docker_images "${_sdir}" ui-legacy-build-image || return 1
         fi
         status_stage "==> Building UI"
         build_ui_legacy "${_sdir}" "${_image}" || return 1
         ;;
      * )
         err_usage "ERROR: Unknown command: '${_command}'"
         return 1
         ;;
   esac
   
   return 0
}

main "$@"
exit $?
