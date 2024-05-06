#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

repository_name=${1}
target_dirs="${@:2}"

if [ -z "$repository_name" ] || [ -z "$target_dirs" ]; then
    echo "Error: repository_name and at least one value for target_dirs must be non-empty strings."
    exit 1
fi

function copy_file {
  for dir in $target_dirs; do
      mkdir -p $dir
      target_path="$dir/${2}"
      echo "-> Copying $1 to $target_path"
      cp "${1}" $target_path
  done
}

if [[ $repository_name == *"-enterprise" ]]; then
    echo "Copying legal (Ent)"
    echo "Downloading EULA.txt and TermsOfEvaluation.txt"
    curl -so /tmp/EULA.txt https://eula.hashicorp.com/EULA.txt
    curl -so /tmp/TermsOfEvaluation.txt https://eula.hashicorp.com/TermsOfEvaluation.txt
    copy_file /tmp/EULA.txt EULA.txt
    copy_file /tmp/TermsOfEvaluation.txt TermsOfEvaluation.txt
else
    echo "Copying legal (CE)"
    copy_file LICENSE LICENSE.txt
fi
