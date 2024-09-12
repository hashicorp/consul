#! /bin/bash

set -eo pipefail

curdir=$(pwd)
if [[ ! $curdir == *"enterprise"* ]]; then
  echo "This script should be run from the enterprise directory."
  exit 1
fi

LAST_RELEASE_GIT_TAG="$1"

if [ -z "$LAST_RELEASE_GIT_TAG" ]; then
  read -p "Enter the last release git tag (vX.Y.Z): " LAST_RELEASE_GIT_TAG
fi

if [ -z "$LAST_RELEASE_GIT_TAG" ]; then
  echo "Last release git tag is required."
  exit 1
fi

if [[ ! $LAST_RELEASE_GIT_TAG == v* ]]; then
  echo "Last release git tag should start with 'v'."
  exit 1
fi

go run github.com/hashicorp/go-changelog/cmd/changelog-build@latest -last-release ${LAST_RELEASE_GIT_TAG}+ent -entries-dir .changelog/ -changelog-template .changelog/changelog.tmpl -note-template .changelog/note.tmpl -this-release $(git rev-parse HEAD)
