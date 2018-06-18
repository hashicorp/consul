function hashicorp_release {
   # Arguments:
   #   $1 - Path to directory containing all of the release artifacts
   #
   # Returns:
   #   0 - success
   #   * - failure
   #
   # Notes:
   #   Requires the AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables
   #   to be set
   
   status "Uploading files"
   hc-releases upload "${1}" || return 1
   
   status "Publishing the release"
   hc-releases publish || return 1
   
   return 0
}

function confirm_git_push_changes {
   # Arguments:
   #   $1 - Path to git repo
   #
   # Returns:
   #   0 - success
   #   * - error
   #
   
   if ! test -d "$1"
   then
      err "ERROR: '$1' is not a directory. confirm_git_push_changes must be called with the path to a git repo as the first argument'" 
      return 1
   fi
   
   pushd "${1}" > /dev/null
   
   
   declare -i ret=0
   git_log_summary || ret=1
   if test ${ret} -eq 0
   then
      # put a empty line between the git changes and the prompt
      echo ""
      
      local answer=""
      
      while true
      do
         case "${answer}" in
            [yY]* )
               status "Changes Accepted"
               ret=0
               break
               ;;
            [nN]* )
               err "Changes Rejected"
               ret=1
               break
               ;;
            * )
               read -p "Are these changes correct? [y/n]: " answer
               ;;
         esac
      done
   fi
   
   popd > /dev/null
   return $ret
}

function confirm_consul_version {
   # Arguments:
   #   $1 - Path to the release files
   #   $2 - Version to look for
   #
   # Returns:
   #   0 - success
   #   * - error
   
   local zfile="${1}/${CONSUL_PKG_NAME}_${2}_$(go env GOOS)_$(go env GOARCH).zip"
   
   if ! test -f "${zfile}"
   then
      err "ERROR: File not found or is not a regular file: ${zfile}"
      return 1
   fi
   
   local ret=0
   local tfile="$(mktemp) -t "${CONSUL_PKG_NAME}_")"
   
   unzip -p "${zfile}" "consul" > "${tfile}"
   if test $? -eq 0
   then
      chmod +x "${tfile}"
      "${tfile}" version
      
      # put a empty line between the version output and the prompt
      echo ""
      
      local answer=""
      
      while true
      do
         case "${answer}" in
            [yY]* )
               status "Version Accepted"
               ret=0
               break
               ;;
            [nN]* )
               err "Version Rejected"
               ret=1
               break
               ;;
            * )
               read -p "Is this Consul version correct? [y/n]: " answer
               ;;
         esac
      done
   else
      err "ERROR: Failed to extract consul binary from the zip file"
      ret=1
   fi
   
   rm "${tfile}" > /dev/null 2>&1
   return ${ret}      
}


function publish_release {
   # Arguments:
   #   $1 - Path to top level Consul source that contains the built release
   #   $2 - boolean whether to publish to git upstream
   #   $3 - boolean whether to publish to releases.hashicorp.com
   #
   # Returns:
   #   0 - success
   #   * - error
   
   if ! test -d "$1"
   then
      err "ERROR: '$1' is not a directory. publish_release must be called with the path to the top level source as the first argument'" 
      return 1
   fi
   
   local sdir="$1"
   local pub_git="$2"
   local pub_hc_releases="$3"
   
   if test -z "${pub_git}"
   then
      pub_git=1
   fi
   
   if test -z "${pub_hc_releases}"
   then
      pub_hc_releases=1
   fi
   
   local vers="$(get_version ${sdir} true false)"
   if test $? -ne 0
   then
      err "Please specify a version (couldn't parse one from the source)." 
      return 1
   fi
   
   status_stage "==> Verifying release files"
   check_release "${sdir}/pkg/dist" "${vers}" true || return 1
   
   status_stage "==> Confirming Consul Version"
   confirm_consul_version "${sdir}/pkg/dist" "${vers}" || return 1
   
   status_stage "==> Confirming Git is clean"
   is_git_clean "$1" true || return 1

   status_stage "==> Confirming Git Changes"
   confirm_git_push_changes "$1" || return 1
   
   if is_set "${pub_git}"
   then
      status_stage "==> Pushing to Git"
      git_push_ref "$1" || return 1
      git_push_ref "$1" "v${vers}" || return 1
   fi
   
   if is_set "${pub_hc_releases}"
   then
      status_stage "==> Publishing to releases.hashicorp.com"
      hashicorp_release "${sdir}/pkg/dist" || return 1
   fi
   
   return 0
}