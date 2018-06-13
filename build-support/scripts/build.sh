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

function can_parse_option {
   local allowed="$1"
   local command="$2"
   local options="$3"
   
   if test ${allowed} -ne 1
   then
      err "ERROR: subcommand ${command} does not support the ${options} options"
      return 1
   fi
   return 0
}

function check_duplicate {
   local is_dup="$1"
   local command="$2"
   local options="$3"
   
   if test ${is_dup} -ne 0
   then
      err "ERROR: options ${options} may not be given more than once to the subcommand ${command}"
      return 1
   fi
   return 0   
}

function option_check {
   can_parse_option "$1" "$3" "$4" && check_duplicate "$2" "$3" "$4"
   return $?
}

function get_option_value {
   # Arguments:
   #   $1 - bool whether the option should be allowed
   #   $2 - bool whether the option has been specified already
   #   $3 - the option value
   #   $4 - the command being executed
   #   $5 - the option names to use for logging
   #
   # Returns:
   #   0 - success
   #   * - failure
   
   option_check "$1" "$2" "$4" "$5" || return 1
   
   if test -z "$3"
   then
      err "ERROR: options ${5} for subcommand ${4} require an argument but none was provided"
      return 1
   fi
   
   echo "$3"
   return 0
}

function usage {
cat <<-EOF
Usage: ${SCRIPT_NAME} <subcommand> [<options ...>]
   
Subcommands:
      assetfs:       Builds the bindata_assetfs.go file from previously build UI artifacts
      
         Options:
            -i | --image      IMAGE       Alternative Docker image to run the build within.
                                          Defaults to ${GO_BUILD_CONTAINER_DEFAULT}
                                        
            -s | --source     DIR         Path to source to build.
                                          Defaults to "${SOURCE_DIR}"
                                          
            -r | --refresh                Enables refreshing the docker image prior to building.
         
      consul:        Builds the main Consul binary. This assumes the assetfs is up to date:
      
         Options:
            -i | --image      IMAGE       Alternative Docker image to run the build within.
                                          Defaults to ${GO_BUILD_CONTAINER_DEFAULT}
                                        
            -s | --source     DIR         Path to source to build.
                                          Defaults to "${SOURCE_DIR}"
                                           
            -r | --refresh                Enables refreshing the docker image prior to building.
      
      consul-local:  Builds the main Consul binary on the local system (no docker)
                                        
            -s | --source     DIR         Path to source to build.
                                          Defaults to "${SOURCE_DIR}"
                                          
            -o | --build-os   OS          Space separated string of OSes to build
            
            -a | --build-arch ARCH        Space separated string of architectures to build
      
      publish:       Publishes a release build.
      
            -s | --source     DIR         Path to the source to build.
                                          Defaults to "${SOURCE_DIR}"
      
      release:       Performs a release build.
      
         Options:
            -s | --source     DIR         Path to source to build.
                                          Defaults to "${SOURCE_DIR}"
         
            -t | --tag        BOOL        Whether to add a release commit and tag the build
                                          Defaults to 1.
                                          
            -b | --build      BOOL        Whether to perform the build of the ui's, assetfs and
                                          binaries. Defaults to 1.
                                          
            -S | --sign       BOOL        Whether to sign the generated SHA256SUMS file.
                                          Defaults to 1.
                                                      
            -g | --gpg-key    KEY         Alternative GPG key to use for signing operations.
                                          Defaults to ${HASHICORP_GPG_KEY}
                  
      ui:            Builds the latest UI.
      
         Options:
            -i | --image      IMAGE       Alternative Docker image to run the build within.
                                          Defaults to ${UI_BUILD_CONTAINER_DEFAULT}
                                     
            -s | --source     DIR         Path to source to build.
                                          Defaults to "${SOURCE_DIR}"
                                          
            -r | --refresh                Enables refreshing the docker image prior to building.
         
      ui-legacy:     Builds the legacy UI
         
         Options:
            -i | --image      IMAGE       Alternative Docker image to run the build within.
                                          Defaults to ${UI_LEGACY_BUILD_CONTAINER_DEFAULT}
                                        
            -s | --source     DIR         Path to source to build.
                                          Defaults to "${SOURCE_DIR}"
                                          
            -r | --refresh                Enables refreshing the docker image prior to building.
         
      version:    Prints out the version parsed from source.
      
         Options:
            -s | --source     DIR         Path to source to build.
                                          Defaults to "${SOURCE_DIR}"
EOF
}

function main {
   declare    build_fn
   declare    sdir
   declare    image
   declare -i refresh_docker=0
   declare -i rel_tag
   declare -i rel_build
   declare -i rel_sign
   declare    rel_gpg_key=""   
   declare    build_os
   declare    build_arch
   declare -i vers_release
   declare -i vers_git

   declare -i use_refresh=1
   declare -i default_refresh=0
   declare -i use_sdir=1
   declare    default_sdir="${SOURCE_DIR}"
   declare -i use_image=0
   declare    default_image=""
   declare -i use_rel=0
   declare -i default_rel_tag=1
   declare -i default_rel_build=1
   declare -i default_rel_sign=1
   declare    default_rel_gpg_key="${HASHICORP_GPG_KEY}"
   declare -i use_xc=0
   declare    default_build_os=""
   declare    default_build_arch=""
   declare -i use_version_args
   declare -i default_vers_rel=0 
   declare -i default_vers_git=0
   
   declare    command="$1"
   shift
   
   case "${command}" in
      assetfs )
         use_image=1
         default_image="${GO_BUILD_CONTAINER_DEFAULT}"         
         ;;
      consul )      
         use_image=1
         default_image="${GO_BUILD_CONTAINER_DEFAULT}"         
         ;;
      consul-local )
         use_xc=1
         ;;
      publish )
         use_refresh=0
         ;;
      release )
         use_rel=1
         use_refresh=0
         ;;
      ui )
         use_image=1
         default_image="${UI_BUILD_CONTAINER_DEFAULT}"         
         ;;
      ui-legacy )
         use_image=1
         default_image="${UI_LEGACY_BUILD_CONTAINER_DEFAULT}"         
         ;;
      version )
         use_refresh=0
         use_version_args=1
         ;;
      -h | --help)
         usage
         return 0
         ;;
      *)
         err "Unkown subcommand: '$1' - possible values are 'consul', 'ui', 'ui-legacy', 'assetfs', version' and 'release'" 
         return 1
         ;;
   esac      
      
   declare -i have_image_arg=0
   declare -i have_sdir_arg=0
   declare -i have_rel_tag_arg=0
   declare -i have_rel_build_arg=0
   declare -i have_rel_sign_arg=0
   declare -i have_rel_gpg_key_arg=0
   declare -i have_refresh_arg=0
   declare -i have_build_os_arg=0
   declare -i have_build_arch_arg=0
   declare -i have_vers_rel_arg=0
   declare -i have_vers_git_arg=0
   
   while test $# -gt 0
   do 
      case $1 in
         -h | --help )
            usage
            return 0
            ;;
         -o | --build-os )
            build_os=$(get_option_value "${use_xc}" "${have_build_os_arg}" "$2" "${command}" "-o/--xc-os") || return 1
            have_build_os_arg=1
            shift 2
            ;;
         -a | --build-arch)
            build_arch=$(get_option_value "${use_xc}" "${have_build_arch_arg}" "$2" "${command}" "-o/--xc-arch") || return 1
            have_build_arch_arg=1
            shift 2
            ;;
         -R | --release )
            option_check "${use_version_args}" "${have_vers_rel_arg}" "${command}" "-R/--release" || return 1
            have_vers_rel_arg=1
            vers_release=1
            shift
            ;;
         -G | --git )
            option_check "${use_version_args}" "${have_vers_git_arg}" "${command}" "-G/--git" || return 1
            have_vers_git_arg=1
            vers_git=1
            shift
            ;;
         -r | --refresh)
            option_check "${use_refresh}" "${have_refresh_arg}" "${command}" "-r/--refresh" || return 1
            have_refresh_arg=1
            refresh_docker=1
            shift
            ;;
         -i | --image )
            image=$(get_option_value "${use_image}" "${have_image_arg}" "$2" "${command}" "-i/--image") || return 1
            have_image_arg=1
            shift 2
            ;;
         -s | --source )
            sdir=$(get_option_value "${use_sdir}" "${have_sdir_arg}" "$2" "${command}" "-s/--source") || return 1
            if ! test -d "${sdir}"
            then
               err "ERROR: -s/--source is not a path to a top level directory"
               return 1
            fi
            have_sdir_arg=1
            shift 2
            ;;
         -t | --tag )
            rel_tag=$(get_option_value "${use_rel}" "${have_rel_tag_arg}" "$2" "${command}" "-t/--tag") || return 1
            have_rel_tag_arg=1
            shift 2
            ;;
         -b | --build )
            rel_build=$(get_option_value "${use_rel}" "${have_rel_build_arg}" "$2" "${command}" "-b/--build") || return 1
            have_rel_build_arg=1
            shift 2
            ;;
         -S | --sign )
            rel_sign=$(get_option_value "${use_rel}" "${have_rel_sign_arg}" "$2" "${command}" "-S/--sign") || return 1
            have_rel_sign_arg=1
            shift 2
            ;;
         -g | --gpg-key )
            rel_gpg_key=$(get_option_value "${use_rel}" "${have_rel_gpg_key_arg}" "$2" "${command}" "-g/--gpg-key") || return 1
            shift 2
            ;;
         *)
            err "ERROR: Unknown option '$1' for subcommand ${command}"
            return 1
            ;;
      esac
   done
   
   test $have_image_arg -ne 1 && image="${default_image}"
   test $have_sdir_arg -ne 1 && sdir="${default_sdir}"
   test $have_rel_tag_arg -ne 1 && rel_tag="${default_rel_tag}"
   test $have_rel_build_arg -ne 1 && rel_build="${default_rel_build}"
   test $have_rel_sign_arg -ne 1 && rel_sign="${default_rel_sign}"
   test $have_rel_gpg_key_arg -ne 1 && rel_gpg_key="${default_rel_gpg_key}"
   test $have_refresh_arg -ne 1 && refresh_docker="${default_refresh}"
   test $have_build_os_arg -ne 1 && build_os="${default_build_os}"
   test $have_build_arch_arg -ne 1 && build_arch="${default_build_os}"
   test $have_vers_rel_arg -ne 1 && vers_release="${default_vers_rel}"
   test $have_vers_git_arg -ne 1 && vers_git="${default_vers_git}"
   
    case "${command}" in
       assetfs )
         if is_set "${refresh_docker}"
         then
            status_stage "==> Refreshing Consul build container image"
            export GO_BUILD_TAG="${image}"
            refresh_docker_images ${sdir} go-build-image || return 1
         fi
         status_stage "==> Build Static Assets"
         build_assetfs "${sdir}" "${image}" || return 1
         ;;
      consul )
         if is_set "${refresh_docker}"
         then
            status_stage "==> Refreshing Consul build container image"
            export GO_BUILD_TAG=${image}
            refresh_docker_images ${sdir} go-build-image || return 1
         fi
         status_stage "==> Building Consul"
         build_consul "${sdir}" "" "${image}" || return 1
         ;;
      consul-local )
         build_consul_local "${sdir}" "${build_os}" "${build_arch}" "" || return 1
         ;;
      publish )
         publish_release "${sdir}" true true || return 1
         ;;
      release )
         if is_set "${refresh_docker}"
         then
            refresh_docker_images ${sdir} || return 1
         fi
         build_release "${sdir}" "${rel_tag}" "${rel_build}" "${rel_sign}" "${rel_gpg_key}" || return 1
         ;;
      ui )
         
         if is_set "${refresh_docker}"
         then
            status_stage "==> Refreshing UI build container image"
            export UI_BUILD_TAG=${image}
            refresh_docker_images ${sdir} ui-build-image || return 1
         fi
         status_stage "==> Building UI"
         build_ui "${sdir}" "${image}" || return 1
         ;;
      ui-legacy )
         if is_set "${refresh_docker}"
         then
            status_stage "==> Refreshing Legacy UI build container image"
            export UI_LEGACY_BUILD_TAG=${image}
            refresh_docker_images ${sdir} ui-legacy-build-image || return 1
         fi
         status_stage "==> Building Legacy UI"
         build_ui_legacy "${sdir}" "${image}" || return 1
         ;;
      version )
         parse_version "${sdir}" "${vers_release}" "${vers_git}" || return 1
         ;;
      *)
         err "Unkown subcommand: '$1' - possible values are 'assetfs', consul', 'consul-local' 'publish', 'release', 'ui', 'ui-legacy' and 'version'" 
         return 1
         ;;
   esac
   
   return 0
}

main $@
exit $?