#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


busl_files=$(grep -r 'SPDX-License-Identifier: BUSL' --exclude=./.github/scripts/license_checker.sh .)

# If we do not find a file in .changelog/, we fail the check
if [ -n "$busl_files" ]; then
    echo "Found BUSL occurrences in the PR branch!"
    echo -n "$busl_files"
    exit 1
else
    echo "Did not find any occurrences of BUSL in the PR branch"
    exit 0
fi
