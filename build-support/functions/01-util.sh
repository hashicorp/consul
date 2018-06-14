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
   #   $3 - boolean whether to use GIT_DESCRIBE and GIT_COMMIT environment variables
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
   
   local git_version=""
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
      git_version="${GIT_DESCRIBE}"
      git_commit="${GIT_COMMIT}"
   fi
   
   # Get the main version out of the source file
   version_main=$(awk '$1 == "Version" && $2 == "=" { gsub(/"/, "", $3); print $3 }' < ${vfile})
   release_main=$(awk '$1 == "VersionPrerelease" && $2 == "=" { gsub(/"/, "", $3); print $3 }' < ${vfile})
   
   # try to determine the version if we have build tags
   for tag in "$GOTAGS"
   do
      for vfile in $(ls "${1}/version/version_*.go" 2> /dev/null| sort)
      do
         if grep -q "// +build $tag" $file
         then
            version_main=$(awk '$1 == "Version" && $2 == "=" { gsub(/"/, "", $3); print $3 }' < ${vfile})
            release_main=$(awk '$1 == "VersionPrerelease" && $2 == "=" { gsub(/"/, "", $3); print $3 }' < ${vfile})
         fi
      done
   done
   
   # override the version from source with the value of the GIT_DESCRIBE env var if present
   if test -n "${git_version}"
   then
      version="${git_version}"
   else
      version="${version_main}"
   fi
      
   if is_set "${include_release}"
   then
      # Get the release version out of the source file
      release="${release_main}"
      
      # When no GIT_DESCRIBE env var is present and no release is in the source then we 
      # are definitely in dev mode
      if test -z "${git_version}" -a -z "$release"
      then
         release="dev"
      fi
      
      # Add the release to the version
      if test -n "$release"
      then
         version="${version}-${release}"
         
         # add the git commit to the version
         if test -n "${git_commit}"
         then
            version="${version} (${git_commit})"
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
   #   $3 - Whether to use GIT_DESCRIBE and GIT_COMMIT environment variables
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

function git_upstream {
   # Arguments:
   #   $1 - Path to the git repo (optional - assumes pwd is git repo otherwise)
   #
   # Returns:
   #   0 - success
   #   * - failure
   #
   # Notes:
   #   Echos the current upstream branch to stdout when successful
   
   local gdir="$(pwd)"
   if test -d "$1"
   then
      gdir="$1"
   fi
   
   pushd "${gdir}" > /dev/null

   local ret=0   
   local head="$(git status -b --porcelain=v2 | awk '{if ($1 == "#" && $2 =="branch.upstream") { print $3 }}')" || ret=1
   
   popd > /dev/null
   
   test ${ret} -eq 0 && echo "$head"
   return ${ret}
}

function git_log_summary {
   # Arguments:
   #   $1 - Path to the git repo (optional - assumes pwd is git repo otherwise)
   #
   # Returns:
   #   0 - success
   #   * - failure
   #
   
   local gdir="$(pwd)"
   if test -d "$1"
   then
      gdir="$1"
   fi
   
   pushd "${gdir}" > /dev/null
   
   local ret=0
   
   local head=$(git_branch) || ret=1
   local upstream=$(git_upstream) || ret=1
   local rev_range="${head}...${upstream}"
   
   if test ${ret} -eq 0
   then
      status "Git Changes:"
      git log --pretty=oneline ${rev_range} || ret=1
      
   fi
   return $ret
}

function normalize_git_url {
   url="${1#https://}"
   url="${url#git@}"
   url="${url%.git}"
   url="$(sed -e 's/\([^\/:]*\)[:\/]\(.*\)/\1:\2/' <<< "${url}")"
   echo "$url"
   return 0
}

function find_git_remote {
   # Arguments:
   #   $1 - Path to the top level Consul source
   #
   # Returns:
   #   0 - success
   #   * - error
   #
   # Note:
   #   The remote name to use for publishing will be echoed to stdout upon success
   
   if ! test -d "$1"
   then
      err "ERROR: '$1' is not a directory. find_git_remote must be called with the path to the top level source as the first argument'" 
      return 1
   fi
   
   need_url=$(normalize_git_url "${PUBLISH_GIT_HOST}:${PUBLISH_GIT_REPO}")
   
   pushd "$1" > /dev/null
   
   local ret=1
   for remote in $(git remote)
   do
      url=$(git remote get-url --push ${remote}) || continue
      url=$(normalize_git_url "${url}")
      
      if test "${url}" == "${need_url}"
      then
         echo "${remote}"
         ret=0
         break
      fi
   done
   
   popd > /dev/null
   return $ret
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