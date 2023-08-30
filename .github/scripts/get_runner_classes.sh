#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

#
# This script generates tag-sets that can be used as runs-on: values to select runners.

set -euo pipefail

case "$GITHUB_REPOSITORY" in
    *-enterprise)
        # shellcheck disable=SC2129
        echo "compute-small=['self-hosted', 'linux', 'small']" >> "$GITHUB_OUTPUT"
        echo "compute-medium=['self-hosted', 'linux', 'medium']" >> "$GITHUB_OUTPUT"
        echo "compute-large=['self-hosted', 'linux', 'large']" >> "$GITHUB_OUTPUT"
        # m5d.8xlarge is equivalent to our xl custom runner in CE
        echo "compute-xl=['self-hosted', 'ondemand', 'linux', 'type=m5d.8xlarge']" >> "$GITHUB_OUTPUT"
        ;;
    *)
        # shellcheck disable=SC2129
        echo "compute-small=['custom-linux-s-consul-latest']" >> "$GITHUB_OUTPUT"
        echo "compute-medium=['custom-linux-m-consul-latest']" >> "$GITHUB_OUTPUT"
        echo "compute-large=['custom-linux-l-consul-latest']" >> "$GITHUB_OUTPUT"
        echo "compute-xl=['custom-linux-xl-consul-latest']" >> "$GITHUB_OUTPUT"
        ;;
esac
