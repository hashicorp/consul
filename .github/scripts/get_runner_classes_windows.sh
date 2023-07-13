#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

#
# This script generates tag-sets that can be used as runs-on: values to select runners.

set -euo pipefail

case "$GITHUB_REPOSITORY" in
    *-enterprise)
        # shellcheck disable=SC2129
        echo "compute-small=['self-hosted', 'windows', 'small']" >> "$GITHUB_OUTPUT"
        echo "compute-medium=['self-hosted', 'windows', 'medium']" >> "$GITHUB_OUTPUT"
        echo "compute-large=['self-hosted', 'windows', 'large']" >> "$GITHUB_OUTPUT"
        # m5d.8xlarge is equivalent to our xl custom runner in OSS
        echo "compute-xl=['self-hosted', 'ondemand', 'windows', 'type=m5d.8xlarge']" >> "$GITHUB_OUTPUT"
        ;;
    *)
        # shellcheck disable=SC2129
        echo "compute-small=['custom-windows-s-consul-latest']" >> "$GITHUB_OUTPUT"
        echo "compute-medium=['custom-windows-m-consul-latest']" >> "$GITHUB_OUTPUT"
        echo "compute-large=['custom-windows-l-consul-latest']" >> "$GITHUB_OUTPUT"
        echo "compute-xl=['custom-windows-xl-consul-latest']" >> "$GITHUB_OUTPUT"
        ;;
esac
