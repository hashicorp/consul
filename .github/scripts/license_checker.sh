#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

if [[ ${GITHUB_BASE_REF} == release/1.14.* ]] || [[ ${GITHUB_BASE_REF} == release/1.15.* ]] || [[ ${GITHUB_BASE_REF} == release/1.16.* ]]; then
    busl_files=$(grep -r 'SPDX-License-Identifier: BUSL' . --exclude-dir .github)

    if [ -n "$busl_files" ]; then
        echo "Found BUSL occurrences in the PR branch! (See NET-5258 for details)"
        echo -n "$busl_files"
        exit 1
    else
        echo "Did not find any occurrences of BUSL in the PR branch"
        exit 0
    fi
    echo "The variable starts with release/1.14, release/1.15, or release/1.17."
else
    echo "Skipping BUSL check since ${GITHUB_BASE_REF} not one of release/1.14.*, release/1.15.*, or release/1.16.*."
    exit 0
fi