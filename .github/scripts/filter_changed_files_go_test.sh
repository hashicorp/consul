#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# Get the list of changed files
files_to_check=$(git diff --name-only "$(git merge-base origin/$BRANCH HEAD~)"...HEAD $*)

# Define the directories to check
skipped_directories=("docs/" "ui/" "website/" "grafana/" ".github/")

# Initialize a variable to track directories outside the skipped ones
skip_ci=false

# Loop through the changed files and find directories/files outside the skipped ones
for file_to_check in $files_to_check; do
	file_is_skipped=false
	for dir in "${skipped_directories[@]}"; do
    echo "Checking... $file_to_check"
		if [[ "$file_to_check" == "$dir"* ]] || [[ "$file_to_check" == *.md && "$dir" == *"/" ]]; then
			file_is_skipped=true
			break
		fi
	done
	if [ "$file_is_skipped" = "false" ]; then
		echo -e $file_to_check
		echo "Non doc file(s) changed - skip-ci: $skip_ci"
		echo "skip-ci=$skip_ci" >>"$GITHUB_OUTPUT"
		exit 0 ## if file is outside of the skipped_directory exit script
	fi
done

skip_ci=true
echo -e "$files_to_check"
echo "Only doc file(s) changed - skip-ci: $skip_ci"
echo "skip-ci=$skip_ci" >>"$GITHUB_OUTPUT"