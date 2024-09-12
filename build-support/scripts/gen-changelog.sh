#! /bin/bash

set -eo pipefail

pr_number=$(gh pr list -H "$(git rev-parse --abbrev-ref HEAD)" --json "number" | jq ".[].number")

# create a new changelog file
touch ".changelog/$pr_number.txt"

echo "Created a new changelog file for PR $pr_number."

# prompt user to pick from list of types of changlog from "breaking-change", "security", "feature", "deprecation", or "bug"
echo "Please select the type of change:"
echo "1. breaking-change"
echo "2. security"
echo "3. feature"
echo "4. deprecation"
echo "5. bug"

if [ -z "$1" ]; then
  read -p "Enter your choice: " choice
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

read -ep $'Please enter the changelog message:\n' msg

echo -e "\`\`\`release-note:$type\n$msg\n\`\`\`" >>".changelog/$pr_number.txt"

cat .changelog/$pr_number.txt
