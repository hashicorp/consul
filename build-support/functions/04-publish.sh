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

function push_git_release {
   # Arguments:
   #   $1 - Path to the top level Consul source
   #   $2 - Tag to push
   #
   # Returns:
   #   0 - success
   #   * - error
   
   if ! test -d "$1"
   then
      err "ERROR: '$1' is not a directory. push_git_release must be called with the path to the top level source as the first argument'" 
      return 1
   fi
   
   local sdir="$1"
   local ret=0
   
   # find the correct remote corresponding to the desired repo (basically prevent pushing enterprise to oss or oss to enterprise)
   local remote=$(find_git_remote "${sdir}") || return 1
   local head=$(git_branch "${sdir}") || return 1
   local upstream=$(git_upstream "${sdir}") || return 1
   status "Using git remote: ${remote}"
   
   # upstream branch for this branch does not track the remote we need to push to
   if test "${upstream#${remote}}" == "${upstream}"
   then
      err "ERROR: Upstream branch '${upstream}' does not track the correct remote '${remote}'"
      return 1
   fi
   
   pushd "${sdir}" > /dev/null
   
   status "Pushing local branch ${head} to ${upstream}"
   if ! git push "${remote}"
   then
      err "ERROR: Failed to push to remote: ${remote}"
      ret=1
   fi
   
   status "Pushing tag ${2} to ${remote}"
   if test "${ret}" -eq 0 && ! git push "${remote}" "${2}"
   then
      err "ERROR: Failed to push tag ${2} to ${remote}"
      ret = 1
   fi
   
   popd > /dev/null
   
   
   return $ret
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
   
   status_page "==> Verifying release files"
   check_release "${sdir}/pkg/dist" "${vers}" true
   
   status_stage "==> Confirming Git is clean"
   is_git_clean "$1" true || return 1

   status_stage "==> Confirming Git Changes"
   confirm_git_push_changes "$1" || return 1
   
   if is_set "${pub_git}"
   then
      status_stage "==> Pushing to Git"
      push_git_release "$1" "v${vers}" || return 1
   fi
   
   if is_set "${pub_hc_releases}"
   then
      status_stage "==> Publishing to releases.hashicorp.com"
      hashicorp_release "${sdir}/pkg/dist" || return 1
   fi
   
   return 0
}