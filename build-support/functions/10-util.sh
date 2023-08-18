# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

function err {
   if test "${COLORIZE}" -eq 1
   then
      tput bold
      tput setaf 1
   fi

   echo "$@" 1>&2

   if test "${COLORIZE}" -eq 1
   then
      tput sgr0
   fi
}

function status {
   if test "${COLORIZE}" -eq 1
   then
      tput bold
      tput setaf 4
   fi

   echo "$@"

   if test "${COLORIZE}" -eq 1
   then
      tput sgr0
   fi
}

function status_stage {
   if test "${COLORIZE}" -eq 1
   then
      tput bold
      tput setaf 2
   fi

   echo "$@"

   if test "${COLORIZE}" -eq 1
   then
      tput sgr0
   fi
}

function debug {
   if is_set "${BUILD_DEBUG}"
   then
      if test "${COLORIZE}" -eq 1
      then
         tput setaf 6
      fi
      echo "$@"
      if test "${COLORIZE}" -eq 1
      then
         tput sgr0
      fi
   fi
}

function debug_run {
   debug "$@"
   "$@"
   return $?
}

function print_run {
   echo "$@"
   "$@"
   return $?
}

function sed_i {
   if test "$(uname)" == "Darwin"
   then
      sed -i '' "$@"
      return $?
   else
      sed -i "$@"
      return $?
   fi
}

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

   gpg --list-secret-keys $1 > /dev/null 2>&1
   return $?
}

function parse_version {
   # Arguments:
   #   $1 - Path to the top level Consul source
   #   $2 - boolean value for whether the release version should be parsed from the source
   #   $3 - boolean whether to use GIT_COMMIT environment variable
   #   $4 - boolean whether to omit the version part of the version string. (optional)
   #
   # Return:
   #   0 - success (will write the version to stdout)
   #   * - error   (no version output)
   #
   # Notes:
   #   If the GOTAGS environment variable is present then it is used to determine which
   #   version file to use for parsing.

   local vfile="${1}/version/version.go"

   # ensure the version file exists
   if ! test -f "${vfile}"
   then
      err "Error - File not found: ${vfile}"
      return 1
   fi

   local include_release="$2"
   local use_git_env="$3"
   local omit_version="$4"

   local git_commit=""

   if test -z "${include_release}"
   then
      include_release=true
   fi

   if test -z "${use_git_env}"
   then
      use_git_env=true
   fi

   if is_set "${use_git_env}"
   then
      git_commit="${GIT_COMMIT}"
   fi

   # Get the main version out of the source file
   version_main=$(awk '$1 == "Version" && $2 == "=" { gsub(/"/, "", $3); print $3 }' < ${vfile})
   release_main=$(awk '$1 == "VersionPrerelease" && $2 == "=" { gsub(/"/, "", $3); print $3 }' < ${vfile})


   # try to determine the version if we have build tags
   for tag in "$GOTAGS"
   do
      for vfile in $(find "${1}/version" -name "version_*.go" 2> /dev/null| sort)
      do
         if grep -q "// +build $tag" "${vfile}"
         then
            version_main=$(awk '$1 == "Version" && $2 == "=" { gsub(/"/, "", $3); print $3 }' < ${vfile})
            release_main=$(awk '$1 == "VersionPrerelease" && $2 == "=" { gsub(/"/, "", $3); print $3 }' < ${vfile})
         fi
      done
   done

   local version="${version_main}"

   local rel_ver=""
   if is_set "${include_release}"
   then
      # Default to pre-release from the source
      rel_ver="${release_main}"

      # When no release is in the source then we are definitely in dev mode
      if test -z "${rel_ver}" && is_set "${use_git_env}"
      then
         rel_ver="dev"
      fi

      # Add the release to the version
      if test -n "${rel_ver}" -a -n "${git_commit}"
      then
         rel_ver="${rel_ver} (${git_commit})"
      fi
   fi

   if test -n "${rel_ver}"
   then
      if is_set "${omit_version}"
      then
         echo "${rel_ver}" | tr -d "'"
      else
         echo "${version}-${rel_ver}" | tr -d "'"
      fi
      return 0
   elif ! is_set "${omit_version}"
   then
      echo "${version}" | tr -d "'"
      return 0
   else
      return 1
   fi
}

function get_version {
   # Arguments:
   #   $1 - Path to the top level Consul source
   #   $2 - Whether the release version should be parsed from source (optional)
   #   $3 - Whether to use GIT_COMMIT environment variable
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
      # parse the CE version from version.go
      vers="$(parse_version ${1} ${2} ${3})"
   fi

   if test -z "$vers"
   then
      return 1
   else
      echo $vers
      return 0
   fi
}

function git_branch {
   # Arguments:
   #   $1 - Path to the git repo (optional - assumes pwd is git repo otherwise)
   #
   # Returns:
   #   0 - success
   #   * - failure
   #
   # Notes:
   #   Echos the current branch to stdout when successful

   local gdir="$(pwd)"
   if test -d "$1"
   then
      gdir="$1"
   fi

   pushd "${gdir}" > /dev/null

   local ret=0
   local head="$(git status -b --porcelain=v2 | awk '{if ($1 == "#" && $2 =="branch.head") { print $3 }}')" || ret=1

   popd > /dev/null

   test ${ret} -eq 0 && echo "$head"
   return ${ret}
}


function git_date {
   # Arguments:
   #   $1 - Path to the git repo (optional - assumes pwd is git repo otherwise)
   #
   # Returns:
   #   0 - success
   #   * - failure
   #
   # Notes:
   #   Echos the date of the last git commit in

   local gdir="$(pwd)"
   if test -d "$1"
   then
      gdir="$1"
   fi

   pushd "${gdir}" > /dev/null

   local ret=0

   # it's tricky to do an RFC3339 format in a cross platform way, so we hardcode UTC
   local date_format="%Y-%m-%dT%H:%M:%SZ"
   # we're using this for build date because it's stable across platform builds
   local date="$(TZ=UTC0 git show -s --format=%cd --date=format-local:"$date_format" HEAD)" || ret=1

   ##local head="$(git status -b --porcelain=v2 | awk '{if ($1 == "#" && $2 =="branch.head") { print $3 }}')" || ret=1

   popd > /dev/null

   test ${ret} -eq 0 && echo "$date"
   return ${ret}
}


function is_git_clean {
   # Arguments:
   #   $1 - Path to git repo
   #   $2 - boolean whether the git status should be output when not clean
   #
   # Returns:
   #   0 - success
   #   * - error
   #

   if ! test -d "$1"
   then
      err "ERROR: '$1' is not a directory. is_git_clean must be called with the path to a git repo as the first argument'"
      return 1
   fi

   local output_status="$2"

   pushd "${1}" > /dev/null

   local ret=0
   test -z "$(git status --porcelain=v2 2> /dev/null)" || ret=1

   if is_set "${output_status}" && test "$ret" -ne 0
   then
      err "Git repo is not clean"
      # --porcelain=v1 is the same as --short except uncolorized
      git status --porcelain=v1
   fi
   popd > /dev/null
   return ${ret}
}

function update_git_env {
   # Arguments:
   #   $1 - Path to git repo
   #
   # Returns:
   #   0 - success
   #   * - error
   #

   if ! test -d "$1"
   then
      err "ERROR: '$1' is not a directory. is_git_clean must be called with the path to a git repo as the first argument'"
      return 1
   fi

   export GIT_COMMIT=$(git rev-parse --short HEAD)
   export GIT_DIRTY=$(test -n "$(git status --porcelain)" && echo "+CHANGES")
   export GIT_IMPORT=github.com/hashicorp/consul/version
   export GIT_DATE=$(git_date "$1")
   export GOLDFLAGS="-X ${GIT_IMPORT}.GitCommit=${GIT_COMMIT}${GIT_DIRTY} -X ${T}.BuildDate=${GIT_DATE}"
   return 0
}

function update_version {
   # Arguments:
   #   $1 - Path to the version file
   #   $2 - Version string
   #   $3 - PreRelease version (if unset will become an empty string)
   #
   # Returns:
   #   0 - success
   #   * - error

   if ! test -f "$1"
   then
      err "ERROR: '$1' is not a regular file. update_version must be called with the path to a go version file"
      return 1
   fi

   if test -z "$2"
   then
      err "ERROR: The version specified was empty"
      return 1
   fi

   local vfile="$1"
   local version="$2"
   local prerelease="$3"

   sed_i ${SED_EXT} -e "s/(Version[[:space:]]*=[[:space:]]*)\"[^\"]*\"/\1\"${version}\"/g" -e "s/(VersionPrerelease[[:space:]]*=[[:space:]]*)\"[^\"]*\"/\1\"${prerelease}\"/g" "${vfile}"
   return $?
}

function set_changelog_version {
   # Arguments:
   #   $1 - Path to top level Consul source
   #   $2 - Version to put into the Changelog
   #   $3 - Release Date
   #
   # Returns:
   #   0 - success
   #   * - error

   local changelog="${1}/CHANGELOG.md"
   local version="$2"
   local rel_date="$3"

   if ! test -f "${changelog}"
   then
      err "ERROR: File not found: ${changelog}"
      return 1
   fi

   if test -z "${version}"
   then
      err "ERROR: Must specify a version to put into the changelog"
      return 1
   fi

   if test -z "${rel_date}"
   then
      rel_date=$(date +"%B %d, %Y")
   fi

   sed_i ${SED_EXT} -e "s/## UNRELEASED/## ${version} (${rel_date})/" "${changelog}"
   return $?
}

function set_website_version {
   # Arguments:
   #   $1 - Path to top level Consul source
   #   $2 - Version to put into the website
   #
   # Returns:
   #   0 - success
   #   * - error

   local config_rb="${1}/website/config.rb"
   local version="$2"

   if ! test -f "${config_rb}"
   then
      err "ERROR: File not found: '${config_rb}'"
      return 1
   fi

   if test -z "${version}"
   then
      err "ERROR: Must specify a version to put into ${config_rb}"
      return 1
   fi

   sed_i ${SED_EXT} -e "s/(h.version[[:space:]]*=[[:space:]]*)\"[^\"]*\"/\1\"${version}\"/g" "${config_rb}"
   return $?
}

function unset_changelog_version {
   # Arguments:
   #   $1 - Path to top level Consul source
   #
   # Returns:
   #   0 - success
   #   * - error

   local changelog="${1}/CHANGELOG.md"

   if ! test -f "${changelog}"
   then
      err "ERROR: File not found: ${changelog}"
      return 1
   fi

   sed_i ${SED_EXT} -e "1 s/^## [0-9]+\.[0-9]+\.[0-9]+ \([^)]*\)/## UNRELEASED/" "${changelog}"
   return $?
}

function set_release_mode {
   # Arguments:
   #   $1 - Path to top level Consul source
   #   $2 - The version of the release
   #   $3 - The release date
   #   $4 - The pre-release version
   #
   #
   # Returns:
   #   0 - success
   #   * - error

   if ! test -d "$1"
   then
      err "ERROR: '$1' is not a directory. set_release_mode must be called with the path to a git repo as the first argument"
      return 1
   fi

   if test -z "$2"
   then
      err "ERROR: The version specified was empty"
      return 1
   fi

   local sdir="$1"
   local vers="$2"
   local rel_date="$(date +"%B %d, %Y")"

   if test -n "$3"
   then
      rel_date="$3"
   fi

   local changelog_vers="${vers}"
   if test -n "$4"
   then
      changelog_vers="${vers}-$4"
   fi

   status_stage "==> Updating CHANGELOG.md with release info: ${changelog_vers} (${rel_date})"
   set_changelog_version "${sdir}" "${changelog_vers}" "${rel_date}" || return 1

   status_stage "==> Updating version/version.go"
   if ! update_version "${sdir}/version/version.go" "${vers}" "$4"
   then
      unset_changelog_version "${sdir}"
      return 1
   fi

   # Only update the website when allowed and there is no pre-release version
   if ! is_set "${CONSUL_NO_WEBSITE_UPDATE}" && test -z "$4"
   then
      status_stage "==> Updating website/config.rb"
      if ! set_website_version "${sdir}" "${vers}"
      then
         unset_changelog_version "${sdir}"
         return 1
      fi
   fi

   return 0
}

function gpg_detach_sign {
   # Arguments:
   #   $1 - File to sign
   #   $2 - Alternative GPG key to use for signing
   #
   # Returns:
   #   0 - success
   #   * - failure

   # determine whether the gpg key to use is being overridden
   local gpg_key=${HASHICORP_GPG_KEY}
   if test -n "$2"
   then
      gpg_key=$2
   fi

   gpg --default-key "${gpg_key}" --detach-sig --yes -v  "$1"
   return $?
}

function shasum_directory {
   # Arguments:
   #   $1 - Path to directory containing the files to shasum
   #   $2 - File to output sha sums to
   #
   # Returns:
   #   0 - success
   #   * - failure

   if ! test -d "$1"
   then
      err "ERROR: '$1' is not a directory and shasum_release requires passing a directory as the first argument"
      return 1
   fi

   if test -z "$2"
   then
      err "ERROR: shasum_release requires a second argument to be the filename to output the shasums to but none was given"
      return 1
   fi

   pushd $1 > /dev/null
   shasum -a256 * > "$2"
   ret=$?
   popd >/dev/null

   return $ret
}

function ui_version {
   # Arguments:
   #   $1 - path to index.html
   #
   # Returns:
   #   0 - success
   #   * -failure
   #
   # Notes: echoes the version to stdout upon success
   if ! test -f "$1"
   then
      err "ERROR: No such file: '$1'"
      return 1
   fi

   local ui_version="$(grep '<!-- CONSUL_VERSION: .* -->' "$1" | sed 's/<!-- CONSUL_VERSION: \(.*\) -->/\1/' | xargs)" || return 1
   echo "$ui_version"
   return 0
}

function ui_logo_type {
   # Arguments:
   #   $1 - path to index.html
   #
   # Returns:
   #   0 - success
   #   * -failure
   #
   # Notes: echoes the 'logo type' to stdout upon success
   # the 'logo' can be one of 'enterprise' or 'oss'
   # and doesn't necessarily correspond to the binary type of consul
   # the logo is 'enterprise' if the binary type is anything but 'oss'
   if ! test -f "$1"
   then
      err "ERROR: No such file: '$1'"
      return 1
   fi
   grep -q "data-enterprise-logo" < "$1"

   if test $? -eq 0
   then
     echo "enterprise"
   else
     # TODO(spatel): CE refactor
     echo "oss"
   fi
   return 0
}

function go_mod_assert {
   # Returns:
   #   0 - success
   #   * - failure
   #
   # Notes: will ensure all the necessary go modules are cached
   # and if the CONSUL_MOD_VERIFY env var is set will force
   # reverification of all modules.
   if ! go mod download >/dev/null
   then
      err "ERROR: Failed to populate the go module cache"
      return 1
   fi

   if is_set "${CONSUL_MOD_VERIFY}"
   then
      if ! go mod verify
      then
         err "ERROR: Failed to verify go module checksums"
         return 1
      fi
   fi
   return 0
}
