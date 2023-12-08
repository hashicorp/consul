#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# Get the list of changed files
# Using `git merge-base` ensures that we're always comparing against the correct branch point. 
#For example, given the commits:
#
# A---B---C---D---W---X---Y---Z # origin/main
#             \---E---F         # feature/branch
#
# ... `git merge-base origin/$SKIP_CHECK_BRANCH HEAD` would return commit `D`
# `...HEAD` specifies from the common ancestor to the latest commit on the current branch (HEAD)..
files_to_check=$(git diff --name-only "$(git merge-base origin/$SKIP_CHECK_BRANCH HEAD~)"...HEAD)

# Define the directories to check
skipped_directories=("docs/" "ui/" "website/" "grafana/")

# Loop through the changed files and find directories/files outside the skipped ones
for file_to_check in "${files_to_check[@]}"; do
	file_is_skipped=false
	for dir in "${skipped_directories[@]}"; do
		if [[ "$file_to_check" == "$dir"* ]] || [[ "$file_to_check" == *.md && "$dir" == *"/" ]]; then
			file_is_skipped=true
			break
		fi
	done
	if [ "$file_is_skipped" != "true" ]; then
		echo -e $file_to_check
        SKIP_CI=false
		echo "Changes detected in non-documentation files - skip-ci: $SKIP_CI"
        echo "skip-ci=$SKIP_CI" >> "$GITHUB_OUTPUT"
		exit 0 ## if file is outside of the skipped_directory exit script
	fi
done

echo -e "$files_to_check"
SKIP_CI=true
echo "Changes detected in only documentation files - skip-ci: $SKIP_CI"
echo "skip-ci=$SKIP_CI" >> "$GITHUB_OUTPUT"