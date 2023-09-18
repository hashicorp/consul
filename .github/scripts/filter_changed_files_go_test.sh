#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# Get the list of changed files
files_to_check=$(git diff --name-only "$(git merge-base origin/$BRANCH HEAD~)"...HEAD $*)

skipped_directories=("docs/" "ui/" "website/" "grafana/")

all_files_in_skipped_dirs=true
skip_ci=false

# Loop through the files to check
for file in "${files_to_check[@]}"; do
    file_directory="$(dirname "$file")"

    # Check if the file is not in the list of skipped directories
    if [[ ! " ${skipped_directories[@]} " =~ " ${file_directory} " ]]; then
        all_files_in_skipped_dirs=false
        break
    fi
done

if [ "$all_files_in_skipped_dirs" = true ]; then
    skip_ci=true
    echo "Only doc file(s) changed - skip-ci: $skip_ci"
    echo "skip-ci=$skip_ci" >>"$GITHUB_OUTPUT"
else
    echo "Non doc file(s) changed - skip-ci: $skip_ci"
    echo "skip-ci=$skip_ci" >>"$GITHUB_OUTPUT"
fi
