#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# check if there is a diff in the xds testdata directory after running `make envoy-regen`
echo "regenerating xds files"
make envoy-regen

echo "calculating changed files"
changed_xds_files=$(git --no-pager diff --name-only HEAD "$(git merge-base HEAD "origin/$GITHUB_BRANCH_REF")" | egrep "agent/xds/testdata/.*" || true)
# If we do not find a file in .changelog/, we fail the check
if [ -z "$changed_xds_files" ]; then
  # pass status check if no changes were found for xds files
  echo "Found no changes to xds golden files"
  exit 0
else
  echo "Found diffs with xds golden files run 'make envoy-regen' to update them and check that output is expected"
  exit 0
fi
