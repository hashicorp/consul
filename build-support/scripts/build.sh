#!/bin/bash
pushd $(dirname ${BASH_SOURCE[0]}) > /dev/null
SCRIPT_DIR=$(pwd)
pushd ../.. > /dev/null
SOURCE_DIR=$(pwd)
popd > /dev/null
popd > /dev/null

source "${SCRIPT_DIR}/functions.sh"

function main {
   case "$1" in
      consul )
         build_consul "${SOURCE_DIR}" "${GO_BUILD_TAG}"
         return $?
         ;;
      ui )
         build_ui "${SOURCE_DIR}" "${UI_BUILD_TAG}"
         return $?
         ;;
      ui-legacy )
         build_ui_legacy "${SOURCE_DIR}" "${UI_LEGACY_BUILD_TAG}"
         return $?
         ;;
      version )
         parse_version "${SOURCE_DIR}"
         return $?
         ;;
      assetfs )
         build_assetfs "${SOURCE_DIR}" "${GO_BUILD_TAG}"
         return $?
         ;;
      *)
         echo "Unkown build: '$1' - possible values are 'consul', 'ui', 'ui-legacy', 'version' and 'assetfs'" 1>&2
         return 1
   esac
}

main $@
exit $?