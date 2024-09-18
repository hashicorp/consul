#! /bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -eo pipefail

pr_number=$(gh pr list -H "$(git rev-parse --abbrev-ref HEAD)" -q ".[0].number" --json "number")

if [ -z "$pr_number" ]; then
  echo "Error: Could not find PR number."
  exit 1
fi

# check if this changelog is referencing an enterprise change
curdir=$(pwd)

filename=".changelog/$pr_number.txt"
if [[ ! $curdir == *"enterprise"* ]]; then
  is_enterprise="n"
  read -rp "Is this an enterprise PR? (y/n): " is_enterprise

  if [[ $is_enterprise == "y" ]]; then
    filename=".changelog/_$pr_number.txt"
  fi
else
  filename=".changelog/_$pr_number.txt"
fi

# create a new changelog file
touch "$filename"

echo "Created a new changelog file for PR $pr_number."

# prompt user to pick from list of types of changlog from "breaking-change", "security", "feature", "deprecation", or "bug"
echo "Please select the type of change:"
echo "1. breaking-change"
echo "2. security"
echo "3. feature"
echo "4. deprecation"
echo "5. bug"

if [ -z "$1" ]; then
  read -rp "Enter your choice: " choice
else
  choice=$1
fi

type=""

case $choice in
1)
  type="breaking-change"
  ;;
2)
  type="security"
  ;;
3)
  type="feature"
  ;;
4)
  type="deprecation"
  ;;
5)
  type="bug"
  ;;
*)
  echo "Invalid choice. Please select a number from 1 to 5."
  exit 1
  ;;
esac

msg=""

read -erp $'Please enter the changelog message:\n' msg

echo -e "\`\`\`release-note:$type\n$msg\n\`\`\`" >>"$filename"

echo -e "\nChangelog added to $filename. Contents:\n"

cat "$filename"
