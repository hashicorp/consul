#!/bin/bash

# Get the list of changed files
files_to_check=$(git diff --name-only origin/$GITHUB_BASE_REF)

# Define the directories to check
skipped_directories=("docs/" "ui/" "website/" "grafana/")

# Initialize a variable to track directories outside the skipped ones
other_directories=""
trigger_ci=false

# Loop through the changed files and find directories/files outside the skipped ones
for file_to_check in $files_to_check; do
	file_is_skipped=false
	for dir in "${skipped_directories[@]}"; do
		if [[ "$file_to_check" == "$dir"* ]] || [[ "$file_to_check" == *.md && "$dir" == *"/" ]]; then
			file_is_skipped=true
			break
		fi
	done
	if [ "$file_is_skipped" = "false" ]; then
		other_directories+="$(dirname "$file_to_check")\n"
		trigger_ci=true
		echo "Non doc file(s) changed - triggered ci: $trigger_ci"
		echo -e $other_directories
		echo "trigger-ci=$trigger_ci" >>"$GITHUB_OUTPUT"
		exit 0 ## if file is outside of the skipped_directory exit script
	fi
done

echo "Only doc file(s) changed - triggered ci: $trigger_ci"
echo "trigger-ci=$trigger_ci" >>"$GITHUB_OUTPUT"
