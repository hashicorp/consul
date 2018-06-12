function err {
   if test "${COLORIZE}" -eq 1
   then
      tput bold
      tput setaf 1
   fi
      
   echo $@ 1>&2
   
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
   
   echo $@
   
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
   
   echo $@
   
   if test "${COLORIZE}" -eq 1
   then
      tput sgr0
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
   #   If the GOTAGS environment variable is present then it is used to determine which
   #   version file to use for parsing.
   #   If the GIT_DESCRIBE environment variable is present then it is used as the version
   #   If the GIT_COMMIT environment variable is preset it will be added to the end of
   #   the version string.
   
   local vfile="${1}/version/version.go"
   
   # ensure the version file exists
   if ! test -f "${vfile}"
   then
      err "Error - File not found: ${vfile}"
      return 1
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
   
   version=
   
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
   fi
   
   if test -z "$vers"
   then
      return 1
   else
      echo $vers
      return 0
   fi
}