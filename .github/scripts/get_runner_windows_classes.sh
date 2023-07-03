#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

#
# This script generates tag-sets that can be used as runs-on: values to select runners.

set -euo pipefail

# shellcheck disable=SC2129
echo "compute-small=['self-hosted', 'windows-2019', 'small']" >> "$GITHUB_OUTPUT"
echo "compute-medium=['self-hosted', 'windows-2019', 'medium']" >> "$GITHUB_OUTPUT"
echo "compute-large=['self-hosted', 'windows-2019', 'large']" >> "$GITHUB_OUTPUT"
echo "compute-xl=['self-hosted', 'ondemand', 'windows-2019', 'type=m5d.8xlarge']" >> "$GITHUB_OUTPUT"
;;