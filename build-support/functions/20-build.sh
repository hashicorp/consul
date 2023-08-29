# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

function supported_osarch {
   # Arguments:
   #   $1 - osarch - example, linux/amd64
   #
   # Returns:
   #   0 - supported
   #   * - not supported
   local osarch="$1"
   for valid in $(go tool dist list)
   do
      if test "${osarch}" = "${valid}"
      then
         return 0
      fi
   done
   return 1
}

function refresh_docker_images {
   # Arguments:
   #   $1 - Path to top level Consul source
   #   $2 - Which make target to invoke (optional)
   #
   # Return:
   #   0 - success
   #   * - failure

   if ! test -d "$1"
   then
      err "ERROR: '$1' is not a directory. refresh_docker_images must be called with the path to the top level source as the first argument'"
      return 1
   fi

   local sdir="$1"
   local targets="$2"

   test -n "${targets}" || targets="docker-images"

   make -C "${sdir}" ${targets}
   return $?
}

function build_ui {
   # Arguments:
   #   $1 - Path to the top level Consul source
   #   $2 - The docker image to run the build within (optional)
   #   $3 - Version override
   #
   # Returns:
   #   0 - success
   #   * - error
   #
   # Notes:
   #   Use the GIT_COMMIT environment variable to pass off to the build
   #   Use the GIT_COMMIT_YEAR environment variable to pass off to the build

   if ! test -d "$1"
   then
      err "ERROR: '$1' is not a directory. build_ui must be called with the path to the top level source as the first argument'"
      return 1
   fi

   local image_name=${UI_BUILD_CONTAINER_DEFAULT}
   if test -n "$2"
   then
      image_name="$2"
   fi

   local sdir="$1"
   local ui_dir="${1}/ui"

   # parse the version
   version=$(parse_version "${sdir}")

   if test -n "$3"
   then
      version="$3"
   fi

   local commit_hash="${GIT_COMMIT}"
   if test -z "${commit_hash}"
   then
      commit_hash=$(git rev-parse --short HEAD)
   fi

   local commit_year="${GIT_COMMIT_YEAR}"
   if test -z "${commit_year}"
   then
      commit_year=$(git show -s --format=%cd --date=format:%Y HEAD)
   fi

   # TODO(spatel): CE refactor
   local logo_type="${CONSUL_BINARY_TYPE}"
   if test "$logo_type" != "oss"
   then
     logo_type="enterprise"
   fi

   # make sure we run within the ui dir
   pushd ${ui_dir} > /dev/null

   status "Creating the UI Build Container with image: ${image_name} and version '${version}'"
   local container_id=$(docker create -it -e "CONSUL_GIT_SHA=${commit_hash}" -e "CONSUL_COPYRIGHT_YEAR=${commit_year}" -e "CONSUL_VERSION=${version}" -e "CONSUL_BINARY_TYPE=${CONSUL_BINARY_TYPE}" ${image_name})
   local ret=$?
   if test $ret -eq 0
   then
      status "Copying the source from '${ui_dir}' to /consul-src within the container"
      (
         tar -c $(ls -A | grep -v "^(node_modules\|dist\|tmp)") | docker cp - ${container_id}:/consul-src &&
         status "Running build in container" && docker start -i ${container_id} &&
         rm -rf ${1}/ui/dist &&
         status "Copying back artifacts" && docker cp ${container_id}:/consul-src/packages/consul-ui/dist ${1}/ui/dist
      )
      ret=$?
      docker rm ${container_id} > /dev/null
   fi

   # Check the version is baked in correctly
   if test ${ret} -eq 0
   then
      local ui_vers=$(ui_version "${1}/ui/dist/index.html")
      if test "${version}" != "${ui_vers}"
      then
         err "ERROR: UI version mismatch. Expecting: '${version}' found '${ui_vers}'"
         ret=1
      fi
   fi

   # Check the logo is baked in correctly
   if test ${ret} -eq 0
   then
     local ui_logo_type=$(ui_logo_type "${1}/ui/dist/index.html")
     if test "${logo_type}" != "${ui_logo_type}"
     then
       err "ERROR: UI logo type mismatch. Expecting: '${logo_type}' found '${ui_logo_type}'"
       ret=1
     fi
   fi

   # Copy UI over ready to be packaged into the binary
   if test ${ret} -eq 0
   then
      rm -rf ${1}/agent/uiserver/dist
      cp -r ${1}/ui/dist ${1}/agent/uiserver/
   fi

   popd > /dev/null
   return $ret
}

function build_consul_post {
   # Arguments
   #   $1 - Path to the top level Consul source
   #   $2 - Subdirectory under pkg/bin (Optional)
   #
   # Returns:
   #   0 - success
   #   * - error
   #
   # Notes:
   #   pkg/bin is where to place binary packages
   #   pkg.bin.new is where the just built binaries are located
   #   bin is where to place the local systems versions

   if ! test -d "$1"
   then
      err "ERROR: '$1' is not a directory. build_consul_post must be called with the path to the top level source as the first argument'"
      return 1
   fi

   local sdir="$1"

   local extra_dir_name="$2"
   local extra_dir=""

   if test -n "${extra_dir_name}"
   then
      extra_dir="${extra_dir_name}/"
   fi

   pushd "${sdir}" > /dev/null

   # recreate the pkg dir
   rm -r pkg/bin/${extra_dir}* 2> /dev/null
   mkdir -p pkg/bin/${extra_dir} 2> /dev/null

   # move all files in pkg.new into pkg
   cp -r pkg.bin.new/${extra_dir}* pkg/bin/${extra_dir}
   rm -r pkg.bin.new

   DEV_PLATFORM="./pkg/bin/${extra_dir}$(go env GOOS)_$(go env GOARCH)"
   for F in $(find ${DEV_PLATFORM} -mindepth 1 -maxdepth 1 -type f 2>/dev/null)
   do
      # recreate the bin dir
      rm -r bin/* 2> /dev/null
      mkdir -p bin 2> /dev/null

      cp ${F} bin/
      cp ${F} ${MAIN_GOPATH}/bin
   done

   popd > /dev/null

   return 0
}

function build_consul {
   # Arguments:
   #   $1 - Path to the top level Consul source
   #   $2 - Subdirectory to put binaries in under pkg/bin (optional - must specify if needing to specify the docker image)
   #   $3 - The docker image to run the build within (optional)
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
      err "ERROR: '$1' is not a directory. build_consul must be called with the path to the top level source as the first argument'"
      return 1
   fi

   local sdir="$1"
   local extra_dir_name="$2"
   local extra_dir=""
   local image_name=${GO_BUILD_CONTAINER_DEFAULT}
   if test -n "$3"
   then
      image_name="$3"
   fi

   pushd ${sdir} > /dev/null
   if is_set "${CONSUL_DEV}"
   then
      if test -z "${XC_OS}"
      then
         XC_OS=$(go env GOOS)
      fi

      if test -z "${XC_ARCH}"
      then
         XC_ARCH=$(go env GOARCH)
      fi
   fi
   XC_OS=${XC_OS:-"solaris darwin freebsd linux windows"}
   XC_ARCH=${XC_ARCH:-"386 amd64 arm arm64"}

   if test -n "${extra_dir_name}"
   then
      extra_dir="${extra_dir_name}/"
   fi

   # figure out if the compiler supports modules
   local use_modules=0
   if go help modules >/dev/null 2>&1
   then
      use_modules=1
   elif test -n "${GO111MODULE}"
   then
      use_modules=1
   fi

   local volume_mount=
   if is_set "${use_modules}"
   then
      status "Ensuring Go modules are up to date"
      # ensure our go module cache is correct
      go_mod_assert || return 1
      # Setup to bind mount our hosts module cache into the container
      volume_mount="--mount=type=bind,source=${MAIN_GOPATH}/pkg/mod,target=/go/pkg/mod"
   fi

   status "Creating the Go Build Container with image: ${image_name}"
   local container_id=$(docker create -it \
      ${volume_mount} \
      -e CGO_ENABLED=0 \
      -e GOLDFLAGS="${GOLDFLAGS}" \
      -e GOTAGS="${GOTAGS}" \
      ${image_name} make linux)
   ret=$?

   if test $ret -eq 0
   then
      status "Copying the source from '${sdir}' to /consul"
      (
         tar -c $(ls | grep -v "^(ui\|website\|bin\|pkg\|.git)") | docker cp - ${container_id}:/consul &&
         status "Running build in container" &&
         docker start -i ${container_id} &&
         status "Copying back artifacts" &&
         docker cp ${container_id}:/consul/pkg/bin pkg.bin.new
      )
      ret=$?
      docker rm ${container_id} > /dev/null

      if test $ret -eq 0
      then
         build_consul_post "${sdir}" "${extra_dir_name}"
         ret=$?
      else
         rm -r pkg.bin.new 2> /dev/null
      fi
   fi
   popd > /dev/null
   return $ret
}
