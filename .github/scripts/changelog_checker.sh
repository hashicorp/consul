#!/bin/bash

set -euo pipefail

# check if there is a diff in the .changelog directory
# for PRs against the main branch, the changelog file name should match the PR number
if [ "$GITHUB_BASE_REF" = "$GITHUB_DEFAULT_BRANCH" ]; then
    enforce_matching_pull_request_number="matching this PR number "
    changelog_file_path=".changelog/(_)?$PR_NUMBER.txt"
else
    changelog_file_path=".changelog/[_0-9]*.txt"
fi

changelog_files=$(git --no-pager diff --name-only HEAD "$(git merge-base HEAD "origin/main")" | egrep "${changelog_file_path}")

# If we do not find a file in .changelog/, we fail the check
if [ -z "$changelog_files" ]; then
    # Fail status check when no .changelog entry was found on the PR
    echo "Did not find a .changelog entry ${enforce_matching_pull_request_number}and the 'pr/no-changelog' label was not applied. Reference - https://github.com/hashicorp/consul/pull/8387"
    exit 1
else
    echo "Found .changelog entry in PR!"
fi
