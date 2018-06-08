# GPG Key ID to use for publically released builds
HASHICORP_GPG_KEY="348FFC4C"

UI_BUILD_CONTAINER_DEFAULT="consul-build-ui"
UI_LEGACY_BUILD_CONTAINER_DEFAULT="consul-build-ui-legacy"

function is_set {
   # Arguments:
   #   $1 - string value to check its truthiness
   #
   # Return:
   #   0 - is truthy (backwards I know but allows syntax like `if is_set <var>` to work)
   #   1 - is not truthy
   
   local val=$(tr '[:upper:]' '[:lower:]' <<< "$1")
   case $val in
      1 | t | true | y | yes)
         return 0
         ;;
      *)
         return 1
         ;;
   esac
}

function have_gpg_key {
   # Arguments:
   #   $1 - GPG Key id to check if we have installed
   #
   # Return:
   #   0 - success (we can use this key for signing)
   #   * - failure (key cannot be used)
   
   gpg --list-secret-keys $1 >dev/null 2>&1
   return $?
}

function parse_version {
   # Arguments:
   #   $1 - Path to the top level Consul source
   #   $2 - boolean value for whether to omit the release version from the version string
   #
   # Return:
   #   0 - success (will write the version to stdout)
   #   * - error   (no version output)
   #
   # Notes:
   #   If the GIT_DESCRIBE environment variable is present then it is used as the version
   #   If the GIT_COMMIT environment variable is preset it will be added to the end of
   #   the version string.
   
   local vfile="${1}/version/version.go"
   
   # ensure the version file exists
   if ! test -f "${vfile}"
   then
      echo "Error - File not found: ${vfile}" 1>&2
      return 1
   fi
   
   # Get the main version out of the source file
   version=$(awk '$1 == "Version" && $2 == "=" { gsub(/"/, "", $3); print $3 }' < ${vfile})
   
   # override the version from source with the value of the GIT_DESCRIBE env var if present
   if test -n "$GIT_DESCRIBE"
   then
      version=$GIT_DESCRIBE
   fi
   
   if ! is_set $2
   then
      # Get the release version out of the source file
      release=$(awk '$1 == "VersionPrerelease" && $2 == "=" { gsub(/"/, "", $3); print $3 }' < ${vfile})
      
      # When no GIT_DESCRIBE env var is present and no release is in the source then we 
      # are definitely in dev mode
      if test -z "$GIT_DESCRIBE" -a -z "$release"
      then
         release="dev"
      fi
      
      # Add the release to the version
      if test -n "$release"
      then
         version="${version}-${release}"
         
         # add the git commit to the version
         if test -n "$GIT_COMMIT"
         then
            version="${version} (${GIT_COMMIT})"
         fi
      fi
   fi
   
   # Output the version
   echo "$version" | tr -d "'"
   return 0
}

function get_version {
   # Arguments:
   #   $1 - Path to the top level Consul source
   #   $2 - Whether the release version should be parsed from source (optional)
   #
   # Returns:
   #   0 - success (the version is also echoed to stdout)
   #   1 - error
   #
   # Notes:
   #   If a VERSION environment variable is present it will override any parsing of the version from the source
   #   In addition to processing the main version.go, version_*.go files will be processed if they have
   #   a Go build tag that matches the one in the GOTAGS environment variable. This tag processing is
   #   primitive though and will not match complex build tags in the files with negation etc.
   
   local vers="$VERSION"
   if test -z "$vers"
   then
      # parse the OSS version from version.go
      vers="$(parse_version ${1} ${2})"
      
      # try to determine the version if we have build tags
      for tag in "$GOTAGS"
      do
         for file in $(ls ${1}/version/version_*.go | sort)
         do
            if grep -q "// +build $tag" $file
            then
               vers=$(awk -F\" '/Version =/ {print $2; exit}' < $file )
            fi
         done
      done
   fi
   
   if test -z "$vers"
   then
      return 1
   else
      echo $vers
      return 0
   fi
}

function tag_release {
   # Arguments:
   #   $1 - Version string to use for tagging the release
   #   $2 - Alternative GPG key id used for signing the release commit (optional)
   #
   # Returns:  
   #   0 - success
   #   * - error
   #
   # Notes:
   #   If the RELEASE_UNSIGNED environment variable is set then no gpg signing will occur
   
   if ! test -d "$1"
   then
      echo "ERROR: '$1' is not a directory. tag_release must be called with the path to the top level source as the first argument'" 1>&2
      return 1
   fi
   
   if test -z "$2"
   then
      echo "ERROR: tag_release must be called with a version number as the second argument" 1>&2
      return 1
   fi
   
   # determine whether the gpg key to use is being overridden
   local gpg_key=${HASHICORP_GPG_KEY}
   if test -n "$3"
   then
      gpg_key=$3
   fi
   
   pushd "$1" > /dev/null
   local ret=0
   
   # perform an usngined release if requested (mainly for testing locally)
   if is_set "$RELEASE_UNSIGNED"
   then
      (
         git commit --allow-empty -a -m "Release v${2}" &&
         git tag -a -m "Version ${2}" "v${2}" master
      )
      ret=$?
   # perform a signed release (official releases should do this)
   elif have_gpg_key ${gpg_key}
   then   
      (
         git commit --allow-empty -a --gpg-sign=${gpg_key} -m "Release v${2}" &&
         git tag -a -m "Version ${2}" -s -u ${gpg_key} "v${2}" master
      )
      ret=$?
   # unsigned release not requested and gpg key isn't useable
   else
      echo "ERROR: GPG key ${gpg_key} is not in the local keychain - to continue set RELEASE_UNSIGNED=1 in the env"
      ret=1
   fi
   popd > /dev/null
   return $ret
}

function build_ui {
   # Arguments:
   #   $1 - Path to the top level Consul source
   #   $2 - The docker image to run the build within (optional)
   #
   # Returns:
   #   0 - success
   #   * - error
   #
   # Notes:
   #   Use the GIT_COMMIT environment variable to pass off to the build
   
   if ! test -d "$1"
   then
      echo "ERROR: '$1' is not a directory. build_ui must be called with the path to the top level source as the first argument'" 1>&2
      return 1
   fi
   
   local image_name=${UI_BUILD_CONTAINER_DEFAULT}
   if test -n "$2"
   then
      image_name="$2"
   fi   
   
   local sdir="$1"
   local ui_dir="${1}/ui-v2"
   
   # parse the version
   version=$(parse_version "${sdir}")
   
   # make sure we run within the ui dir
   pushd ${ui_dir} > /dev/null
   
   echo "Creating the UI Build Container"
   local container_id=$(docker create -it -e "CONSUL_GIT_SHA=${GIT_COMMIT}" -e "CONSUL_VERSION=${version}" ${image_name})
   local ret=$?
   if test $ret -eq 0
   then
      echo "Copying the source from '${ui_dir}' to /consul-src within the container"
      (
         docker cp . ${container_id}:/consul-src &&
         echo "Running build in container" && docker start -i ${container_id} &&
         rm -rf ${1}/ui-v2/dist &&
         echo "Copying back artifacts" && docker cp ${container_id}:/consul-src/dist ${1}/ui-v2/dist
      )
      ret=$?
      docker rm ${container_id} > /dev/null
   fi
   
   if test $ret -eq 0
   then
      rm -rf ${1}/pkg/web_ui/v2
      cp -r ${1}/ui-v2/dist ${1}/pkg/web_ui/v2
   fi
   popd > /dev/null
   return $ret
}

function build_ui_legacy {
   # Arguments:
   #   $1 - Path to the top level Consul source
   #   $2 - The docker image to run the build within (optional)
   #
   # Returns:
   #   0 - success
   #   * - error
    
   if ! test -d "$1"
   then
      echo "ERROR: '$1' is not a directory. build_ui_legacy must be called with the path to the top level source as the first argument'" 1>&2
      return 1
   fi
   
   local sdir="$1"
   local ui_legacy_dir="${sdir}/ui"
   
   local image_name=${UI_LEGACY_BUILD_CONTAINER_DEFAULT}
   if test -n "$2"
   then
      image_name="$2"
   fi   
    
   pushd ${ui_legacy_dir} > /dev/null
   echo "Creating the Legacy UI Build Container"
   rm -r ${sdir}/pkg/web_ui/v1 >/dev/null 2>&1
   mkdir -p ${sdir}/pkg/web_ui/v1
   local container_id=$(docker create -it ${image_name})
   local ret=$?
   if test $ret -eq 0
   then
      echo "Copying the source from '${ui_legacy_dir}' to /consul-src/ui within the container"
      (
         docker cp . ${container_id}:/consul-src/ui &&
         echo "Running build in container" && 
         docker start -i ${container_id} &&
         echo "Copying back artifacts" && 
         docker cp ${container_id}:/consul-src/pkg/web_ui ${sdir}/pkg/web_ui/v1
      )
      ret=$?
      docker rm ${container_id} > /dev/null
   fi
   popd > /dev/null
   return $ret
}

function build_assetfs {
   # Arguments:
   #   $1 - Path to the top level Consul source
   #   $2 - The docker image to run the build within (optional)
   #
   # Returns:
   #   0 - success
   #   * - error
   #
   # Note:
   #   The GIT_COMMIT, GIT_DIRTY and GIT_DESCRIBE environment variables will be used if present
    
   if ! test -d "$1"
   then
      echo "ERROR: '$1' is not a directory. build_assetfs must be called with the path to the top level source as the first argument'" 1>&2
      return 1
   fi
   
   local sdir="$1"
   local image_name=${GO_BUILD_CONTAINER_DEFAULT}
   if test -n "$2"
   then
      image_name="$2"
   fi   
   
   pushd ${sdir} > /dev/null
   echo "Creating the Go Build Container"
   local container_id=$(docker create -it -e GIT_COMMIT=${GIT_COMMIT} -e GIT_DIRTY=${GIT_DIRTY} -e GIT_DESCRIBE=${GIT_DESCRIBE} ${image_name} make static-assets ASSETFS_PATH=bindata_assetfs.go)
   local ret=$?
   if test $ret -eq 0
   then
      echo "Copying the sources from '${sdir}/(pkg|GNUmakefile)' to /go/src/github.com/hashicorp/consul/pkg"
      (
         tar -c pkg/web_ui GNUmakefile | docker cp - ${container_id}:/go/src/github.com/hashicorp/consul &&
         echo "Running build in container" && docker start -i ${container_id} &&
         echo "Copying back artifacts" && docker cp ${container_id}:/go/src/github.com/hashicorp/consul/bindata_assetfs.go ${sdir}/agent/bindata_assetfs.go
      )
      ret=$? 
      docker rm ${container_id} > /dev/null
   fi
   popd >/dev/null
   return $ret   
}

function build_consul {
   # Arguments:
   #   $1 - Path to the top level Consul source
   #   $2 - The docker image to run the build within (optional)
   #
   # Returns:
   #   0 - success
   #   * - error
   #
   # Note:
   #   The GOLDFLAGS and GOTAGS environment variables will be used if set
   #   If the CONSUL_DEV environment var is truthy only the local platform/architecture is built.
   #   If the XC_OS or the XC_ARCH environment vars are present then only those platforms/architectures
   #   will be built. Otherwise all supported platform/architectures are built
    
   if ! test -d "$1"
   then
      echo "ERROR: '$1' is not a directory. build_consul must be called with the path to the top level source as the first argument'" 1>&2
      return 1
   fi
   
   local sdir="$1"
   local image_name=${GO_BUILD_CONTAINER_DEFAULT}
   if test -n "$2"
   then
      image_name="$2"
   fi 
   
   pushd ${sdir} > /dev/null
   echo "Creating the Go Build Container"
   if is_set "${CONSUL_DEV}"
   then
      XC_OS=$(go_env GOOS)
      XC_ARCH=$(go env GOARCH)
   else
      XC_OS=${XC_OS:-"solaris darwin freebsd linux windows"}
      XC_ARCH=${XC_ARCH:-"386 amd64 arm arm64"}
   fi
   
   local container_id=$(docker create -it ${image_name} gox -os="${XC_OS}" -arch="${XC_ARCH}" -osarch="!darwin/arm !darwin/arm64" -ldflags "${GOLDFLAGS}" -output "pkg/{{.OS}}_{{.Arch}}/consul" -tags="${GOTAGS}")
   ret=$?

   if test $ret -eq 0
   then
      echo "Copying the source from '${sdir}' to /go/src/github.com/hashicorp/consul/pkg"
      (
         tar -c $(ls | grep -v "ui\|ui-v2\|website\|bin\|.git") | docker cp - ${container_id}:/go/src/github.com/hashicorp/consul &&
         echo "Running build in container" &&
         docker start -i ${container_id} &&
         echo "Copying back artifacts" &&
         docker cp ${container_id}:/go/src/github.com/hashicorp/consul/pkg/ pkg.new
      )
      ret=$?
      docker rm ${container_id} > /dev/null
      
      DEV_PLATFORM="./pkg.new/$(go env GOOS)_$(go env GOARCH)"
      for F in $(find ${DEV_PLATFORM} -mindepth 1 -maxdepth 1 -type f)
      do
         cp ${F} bin/
         cp ${F} ${GOPATH}/bin
      done
      
      cp -r pkg.new/* pkg/
      rm -r pkg.new
   fi
   popd > /dev/null   
   return $ret
}

function package_release {
   # Arguments:
   #   $1 - Path to the top level Consul source
   #   $2 - Version to use in the names of the zip files (optional)
   #
   # Returns:
   #   0 - success
   #   * - error
   
   if ! test -d "$1"
   then
      echo "ERROR: '$1' is not a directory. package_release must be called with the path to the top level source as the first argument'" 1>&2
      return 1
   fi
   
   local vers="${2}"
   if test -z "${vers}"
   then
      vers=$(get_version $1 false)
      ret=$?
      if test "$ret" -ne 0
      then
         echo "ERROR: failed to determine the version." 1>&2
         return $ret
      fi
   fi
   
   local sdir="$1"
   local ret=0
   for platform in $(find "${sdir}/pkg" -mindepth 1 -maxdepth 1 -type d)
   do
      local os_arch=$(basename $platform)
      pushd "${platform}" > /dev/null
      zip "${sdir}/pkg/dist/consul_${vers}_${os_arch}.zip" ./*
      ret=$?
      popd > /dev/null
      
      if test "$ret" -ne 0
      then
         break
      fi
   done
   
   return $ret
}