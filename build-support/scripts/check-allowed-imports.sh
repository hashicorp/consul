#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


readonly SCRIPT_NAME="$(basename ${BASH_SOURCE[0]})"
readonly SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
readonly SOURCE_DIR="$(dirname "$(dirname "${SCRIPT_DIR}")")"
readonly FN_DIR="$(dirname "${SCRIPT_DIR}")/functions"

source "${SCRIPT_DIR}/functions.sh"


set -uo pipefail

usage() {
cat <<-EOF
Usage: ${SCRIPT_NAME} <module root> [<allowed relative package path>...]

Description:
    Verifies that only the specified packages may be imported from the given module 

Options:
    -h | --help              Print this help text.
EOF
}

function err_usage {
    err "$1"
    err ""
    err "$(usage)"
}

function main {
   local module_root=""
   declare -a allowed_packages=()
   while test $# -gt 0
   do
      case "$1" in
         -h | --help )
         usage
         return 0
         ;;
      * )
         if test -z "$module_root"
         then
            module_root="$1"
         else
            allowed_packages+="$1"
         fi
         shift     
      esac
   done
   
   # If we could guarantee this ran with bash 4.2+ then the final argument could
   # be just ${allowed_packages[@]}. However that with older versions of bash
   # in combination with set -u causes bash to emit errors about using unbound
   # variables when no allowed packages have been specified (i.e. the module should
   # generally be disallowed with no exceptions). This syntax is very strange
   # but seems to be the prescribed workaround I found.
   check_imports "$module_root" ${allowed_packages[@]+"${allowed_packages[@]}"}
   return $?
}

function check_imports {
   local module_root="$1"
   shift
   local allowed_packages="$@"
   
   module_imports=$( go list -test -f '{{join .TestImports "\n"}}' ./... | grep "$module_root" | sort | uniq)
   module_test_imports=$( go list -test -f '{{join .TestImports "\n"}}' ./... | grep "$module_root" | sort | uniq)

   any_error=0
   
   for imp in $module_imports
   do
      is_import_allowed "$imp" "$module_root" $allowed_packages
      allowed=$?
      
      if test $any_error -ne 1
      then
         any_error=$allowed
      fi
   done
  
   if test $any_error -eq 1
   then
      echo "Only the following direct imports are allowed from module $module_root:"
      for pkg in $allowed_packages
      do
         echo "   * $pkg"
      done
   fi

   return $any_error   
}

function is_import_allowed {
   local pkg_import=$1
   shift
   local module_root=$1
   shift
   local allowed_packages="$@"
   
   # check if the import path is a part of the module we are restricting imports for
   if test "$( go list -f '{{.Module.Path}}' $pkg_import)" != "$module_root"
   then
      return 0
   fi
   
   for pkg in $allowed_packages
   do
      if test "${module_root}/$pkg" == "$pkg_import"
      then
         return 0
      fi
   done
   
   err "Import of package $pkg_import is not allowed"
   return 1
}

main "$@"
exit $?