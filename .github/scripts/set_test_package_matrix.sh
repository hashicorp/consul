#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

set -euo pipefail

# set matrix var to list of unique packages containing tests
# TODO: TEMP: only the first 10, for testing
export RUNNER_COUNT=10
matrix="$( \
  go list -json="ImportPath,TestGoFiles" ./... | \
  jq --compact-output '. | select(.TestGoFiles != null) | .ImportPath' | \
  jq --slurp --compact-output '.[0:10]' | \
  jq --argjson runnercount $RUNNER_COUNT  -cM '[_nwise(length / $runnercount | floor)]' \
)"

echo "matrix=${matrix}" >> "${GITHUB_OUTPUT}"
