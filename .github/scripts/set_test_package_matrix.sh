#!/usr/bin/env bash
# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail
export RUNNER_COUNT=$1

if ((RUNNER_COUNT < 2 ))
then
  echo ERROR: RUNNER_COUNT must be greater than 1. Otherwise use the non-split unit test cod in .github/workflows/reusable-unit.yml.
  exit 1 # terminate and indicate error
else
  EFFECTIVE_RUNNER_COUNT=$((RUNNER_COUNT-1))
fi

# set matrix var to list of unique packages containing tests, split into chunks
# of length/runnercount packages (without jq's _nwise helper, removed in jq 1.8)
matrix="$(go list -json="ImportPath,TestGoFiles" ./... | jq --compact-output '. | select(.TestGoFiles != null) | select(.ImportPath != "github.com/hashicorp/consul/agent") | .ImportPath' | shuf | jq --slurp --compact-output '.' | jq --argjson runnercount $EFFECTIVE_RUNNER_COUNT  -cM '(length / $runnercount | ceil | if . < 1 then 1 else . end) as $size | [ range(0; length; $size) as $i | .[$i:$i + $size] ]' | jq --compact-output  '. += [["github.com/hashicorp/consul/agent"]]'))"

echo "matrix=${matrix}" >> "${GITHUB_OUTPUT}"
