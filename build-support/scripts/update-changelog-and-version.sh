#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


readonly SCRIPT_NAME="$(basename ${BASH_SOURCE[0]})"
readonly SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
readonly SOURCE_DIR="$(dirname "$(dirname "${SCRIPT_DIR}")")"
readonly FN_DIR="$(dirname "${SCRIPT_DIR}")/functions"

source "${SCRIPT_DIR}/functions.sh"

function usage {
cat <<-EOF
Usage: ${SCRIPT_NAME}  [<options ...>]

Description:

   This script updates the Consul changelog and version file. It is generally intended for use during release.

Options:
   -v | --version             Current version of the release

   -p | --previous-sha    Previous SHA of the release/1.x.(y-1) in case of a minor release

   -c | --ce-path             Local path to the Consul repo source code

   -h | --help                Print this help text.
EOF
}

function err_usage {
   err "$1"
   err ""
   err "$(usage)"
}

function main {
  declare    sdir="${SOURCE_DIR}"
  declare -i release=0
  declare -i git_info=0
  local version previous_sha ce_path patch_release_only  release_notes header
  echo "Use only in case of  patch release"


  while test $# -gt 0
  do
    case "$1" in
      -h | --help )
        usage
        return 0
        ;;
      -v | --version )
        if test -z "$2"
        then
           err_usage "ERROR: option -v/--version requires an argument"
           return 1
        fi

        version="$2"
        shift 2
        ;;
      -p | --previous-version )
        if test -z "$2"
        then
           err_usage "ERROR: option -p/--previous-version requires an argument"
           return 1
        fi

        previous_sha="$2"
        shift 2
        ;;
      -c | --ce-path )
        if test -z "$2"
        then
          err_usage "ERROR: option -c/--ce-path requires an argument"
          return 1
        fi

        ce_path="$2"
        shift 2
        ;;
       --patch-release )
            patch_release_only=1
            shift 1
            ;;
      *)
        err_usage "ERROR: Unknown argument: '$1'"
        return 1
        ;;
    esac
  done



  # required arguments
  if [ -z "${version}" ]; then
      err_usage "ERROR: version is required"
      return 1
  fi
  if [ -z "${previous_sha}" ]; then
      err_usage "ERROR: previous-version is required"
      return 1
  fi
  if [ -z "${ce_path}" ]; then
      err_usage "ERROR: ce-path is required"
      return 1
  fi

  if [ -z "${patch_release_only}" ]; then
      err_usage "ERROR: Confirm and set --patch-release flag. This script is only for patch releases"
      return 1
  fi

  # Remove the "v" prefix from the version and previous version if it exists.
  version="${version#"v"}"
  previous_sha="${previous_sha#"v"}"

  # -----------------------
  # Update CHANGELOG.md
  # -----------------------
  local changelog_path
  changelog_path="${ce_path}/CHANGELOG.md"

  echo -e "Updating ${changelog_path} for version ${version}...\n"
  # Running it inside of consul-enterprise ensures that all Enterprise-only
  # entries are also generated, just without a pull request link.
  release_notes=$(go run github.com/hashicorp/go-changelog/cmd/changelog-build@latest \
  -last-release "${previous_sha}" \
  -entries-dir .changelog/ \
  -changelog-template .changelog/changelog.tmpl \
  -note-template .changelog/note.tmpl \
  -this-release "$(git rev-parse HEAD)")

  # Add a new header to the top of CHANGELOG.md: "## ${VERSION} (${month} ${day-ordinal}, ${year})" (e.g. ## 1.11.7 (July 13, 2022)).
  header="## ${version}"

  header+=" ($(date +'%B %d, %Y'))"

  echo -e "${header}\n${release_notes}\n\n$(cat "${changelog_path}")" > "${changelog_path}"

  # -----------------------
  # Update version/VERSION
  # -----------------------
  # Update the version file in the consul-enterprise repo.
  echo -e "${version}" > "./version/VERSION"

  echo -e "Finished updating ${changelog_path} and ${ce_path}/version/VERSION in both consul and consul-enterprise repositories. \n"
  echo -e "Please review changes and commit."

  return 0
}

main "$@"
exit $?