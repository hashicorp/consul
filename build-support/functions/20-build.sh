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
   local ui_dir="${1}/ui-v2"
   
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
   local logo_type="${CONSUL_BINARY_TYPE}"
   if test "$logo_type" != "oss"
   then
     logo_type="enterprise"
   fi
   
   # make sure we run within the ui dir
   pushd ${ui_dir} > /dev/null
   
   status "Creating the UI Build Container with image: ${image_name} and version '${version}'"
   local container_id=$(docker create -it -e "CONSUL_GIT_SHA=${commit_hash}" -e "CONSUL_VERSION=${version}" -e "CONSUL_BINARY_TYPE=${CONSUL_BINARY_TYPE}" ${image_name})
   local ret=$?
   if test $ret -eq 0
   then
      status "Copying the source from '${ui_dir}' to /consul-src within the container"
      (
         tar -c $(ls -A | grep -v "^(node_modules\|dist\|tmp)") | docker cp - ${container_id}:/consul-src &&
         status "Running build in container" && docker start -i ${container_id} &&
         rm -rf ${1}/ui-v2/dist &&
         status "Copying back artifacts" && docker cp ${container_id}:/consul-src/dist ${1}/ui-v2/dist
      )
      ret=$?
      docker rm ${container_id} > /dev/null
   fi

   # Check the version is baked in correctly
   if test ${ret} -eq 0
   then
      local ui_vers=$(ui_version "${1}/ui-v2/dist/index.html")
      if test "${version}" != "${ui_vers}"
      then
         err "ERROR: UI version mismatch. Expecting: '${version}' found '${ui_vers}'"
         ret=1
      fi
   fi

   # Check the logo is baked in correctly
   if test ${ret} -eq 0
   then
     local ui_logo_type=$(ui_logo_type "${1}/ui-v2/dist/index.html")
     if test "${logo_type}" != "${ui_logo_type}"
     then
       err "ERROR: UI logo type mismatch. Expecting: '${logo_type}' found '${ui_logo_type}'"
       ret=1
     fi
   fi

   # Copy UI over ready to be packaged into the binary
   if test ${ret} -eq 0
   then
      rm -rf ${1}/pkg/web_ui/v2
      mkdir -p ${1}/pkg/web_ui
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
      err "ERROR: '$1' is not a directory. build_ui_legacy must be called with the path to the top level source as the first argument'" 
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
   status "Creating the Legacy UI Build Container with image: ${image_name}"
   rm -r ${sdir}/pkg/web_ui/v1 >/dev/null 2>&1
   mkdir -p ${sdir}/pkg/web_ui/v1
   local container_id=$(docker create -it ${image_name})
   local ret=$?
   if test $ret -eq 0
   then
      status "Copying the source from '${ui_legacy_dir}' to /consul-src/ui within the container"
      (
         docker cp . ${container_id}:/consul-src/ui &&
         status "Running build in container" && 
         docker start -i ${container_id} &&
         status "Copying back artifacts" && 
         docker cp ${container_id}:/consul-src/pkg/web_ui/v1/. ${sdir}/pkg/web_ui/v1
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
      err "ERROR: '$1' is not a directory. build_assetfs must be called with the path to the top level source as the first argument'" 
      return 1
   fi
   
   local sdir="$1"
   local image_name=${GO_BUILD_CONTAINER_DEFAULT}
   if test -n "$2"
   then
      image_name="$2"
   fi   
   
   pushd ${sdir} > /dev/null
   status "Creating the Go Build Container with image: ${image_name}"
   local container_id=$(docker create -it -e GIT_COMMIT=${GIT_COMMIT} -e GIT_DIRTY=${GIT_DIRTY} -e GIT_DESCRIBE=${GIT_DESCRIBE} ${image_name} make static-assets ASSETFS_PATH=bindata_assetfs.go)
   local ret=$?
   if test $ret -eq 0
   then
      status "Copying the sources from '${sdir}/(pkg/web_ui|GNUmakefile)' to /go/src/github.com/hashicorp/consul/pkg"
      (
         tar -c pkg/web_ui GNUmakefile | docker cp - ${container_id}:/go/src/github.com/hashicorp/consul &&
         status "Running build in container" && docker start -i ${container_id} &&
         status "Copying back artifacts" && docker cp ${container_id}:/go/src/github.com/hashicorp/consul/bindata_assetfs.go ${sdir}/agent/bindata_assetfs.go
      )
      ret=$? 
      docker rm ${container_id} > /dev/null
   fi
   popd >/dev/null
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
   status "Creating the Go Build Container with image: ${image_name}"
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

   local container_id=$(docker create -it -e CGO_ENABLED=0 ${image_name} gox -os="${XC_OS}" -arch="${XC_ARCH}" -osarch="!darwin/arm !darwin/arm64" -ldflags "${GOLDFLAGS}" -output "pkg/bin/${extra_dir}{{.OS}}_{{.Arch}}/consul" -tags="${GOTAGS}")
   ret=$?

   if test $ret -eq 0
   then
      status "Copying the source from '${sdir}' to /go/src/github.com/hashicorp/consul"
      (
         tar -c $(ls | grep -v "^(ui\|ui-v2\|website\|bin\|pkg\|.git)") | docker cp - ${container_id}:/go/src/github.com/hashicorp/consul &&
         status "Running build in container" &&
         docker start -i ${container_id} &&
         status "Copying back artifacts" &&
         docker cp ${container_id}:/go/src/github.com/hashicorp/consul/pkg/bin pkg.bin.new
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

function build_consul_local {
   # Arguments:
   #   $1 - Path to the top level Consul source
   #   $2 - Space separated string of OSes to build. If empty will use env vars for determination.
   #   $3 - Space separated string of architectures to build. If empty will use env vars for determination.
   #   $4 - Subdirectory to put binaries in under pkg/bin (optional)
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
   #   The NOGOX environment variable will be used if present. This will prevent using gox and instead
   #   build with go install
   
   if ! test -d "$1"
   then
      err "ERROR: '$1' is not a directory. build_consul must be called with the path to the top level source as the first argument'" 
      return 1
   fi
   
   local sdir="$1"
   local build_os="$2"
   local build_arch="$3"
   local extra_dir_name="$4"
   local extra_dir=""  
   
   if test -n "${extra_dir_name}"
   then
      extra_dir="${extra_dir_name}/"
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
   
   if test -z "${build_os}"
   then
      build_os="${XC_OS}"
   fi
   
   if test -z "${build_arch}"
   then
      build_arch="${XC_ARCH}"
   fi
   
   local use_gox=1
   is_set "${NOGOX}" && use_gox=0
   which gox > /dev/null || use_gox=0
   
   status_stage "==> Building Consul - OSes: ${build_os}, Architectures: ${build_arch}"
   mkdir pkg.bin.new 2> /dev/null
   if is_set "${use_gox}"
   then 
      status "Using gox for concurrent compilation"
      
      CGO_ENABLED=0 gox \
         -os="${build_os}" \
         -arch="${build_arch}" \
         -osarch="!darwin/arm !darwin/arm64" \
         -ldflags="${GOLDFLAGS}" \
         -output "pkg.bin.new/${extra_dir}{{.OS}}_{{.Arch}}/consul" \
         -tags="${GOTAGS}" \
         .

      if test $? -ne 0
      then
         err "ERROR: Failed to build Consul"
         rm -r pkg.bin.new
         return 1
      fi
   else
      status "Building sequentially with go install"
      for os in ${build_os}
      do
         for arch in ${build_arch}
         do
            outdir="pkg.bin.new/${extra_dir}${os}_${arch}"
            osarch="${os}/${arch}"
            if test "${osarch}" == "darwin/arm" -o "${osarch}" == "darwin/arm64" -o "${osarch}" == "freebsd/arm64" -o "${osarch}" == "windows/arm" -o "${osarch}" == "windows/arm64"
            then
               continue
            fi
            
            if test "${os}" == "solaris" -a "${arch}" != "amd64"
            then
               continue 
            fi
            
            echo "--->   ${osarch}"
            
            
            mkdir -p "${outdir}"
            GOBIN_EXTRA=""
            if test "${os}" != "$(go env GOOS)" -o "${arch}" != "$(go env GOARCH)"
            then
               GOBIN_EXTRA="${os}_${arch}/"
            fi
            binname="consul"
            if [ $os == "windows" ];then
                binname="consul.exe"
            fi
            CGO_ENABLED=0 GOOS=${os} GOARCH=${arch} go install -ldflags "${GOLDFLAGS}" -tags "${GOTAGS}" && cp "${MAIN_GOPATH}/bin/${GOBIN_EXTRA}${binname}" "${outdir}/${binname}"
            if test $? -ne 0
            then
               err "ERROR: Failed to build Consul for ${osarch}"
               rm -r pkg.bin.new
               return 1
            fi
         done
      done
   fi

   build_consul_post "${sdir}" "${extra_dir_name}"
   if test $? -ne 0
   then
      err "ERROR: Failed postprocessing Consul binaries"
      return 1
   fi
   return 0
}
