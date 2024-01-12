#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail
export RUNNER_COUNT=$1

if ((RUNNER_COUNT < 1 ))
then
  echo ERROR: RUNNER_COUNT must be greater than zero.
  exit 1 # terminate and indicate error
elif ((RUNNER_COUNT == 1 ))
then
  EFFECTIVE_RUNNER_COUNT=1
else
  EFFECTIVE_RUNNER_COUNT=$((RUNNER_COUNT-1))
fi

# set matrix var to list of unique packages containing tests
matrix="$(go list -json="ImportPath,TestGoFiles" ./... | jq --compact-output '. | select(.TestGoFiles != null) | select(.ImportPath != "github.com/hashicorp/consul/agent") | .ImportPath' | shuf | jq --slurp --compact-output '.' | jq --argjson runnercount $EFFECTIVE_RUNNER_COUNT  -cM '[_nwise(length / $runnercount | ceil)]' | jq --compact-output  '. += [["github.com/hashicorp/consul/agent"]]'))"

echo "matrix=${matrix}"
#>> "${GITHUB_OUTPUT}"
