#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail
export RUNNER_COUNT=$1

# set matrix var to list of unique packages containing tests
matrix="$(go list -json="ImportPath,TestGoFiles" ./... | jq --compact-output '. | select(.TestGoFiles != null) | .ImportPath' | jq --slurp --compact-output '.' | jq --argjson runnercount $RUNNER_COUNT  -cM '[_nwise(length / $runnercount | floor)]'))"

echo "matrix=${matrix}" >> "${GITHUB_OUTPUT}"
