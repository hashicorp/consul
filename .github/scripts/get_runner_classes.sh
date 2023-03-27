#!/usr/bin/env bash
#
# This script generates tag-sets that can be used as runs-on: values to select runners.

set -euo pipefail

case "$GITHUB_REPOSITORY" in
    *-enterprise)
        # shellcheck disable=SC2129
        echo "compute-small=['self-hosted', 'linux', 'small']" >> "$GITHUB_OUTPUT"
        echo "compute-medium=['self-hosted', 'linux', 'medium']" >> "$GITHUB_OUTPUT"
        echo "compute-large=['self-hosted', 'linux', 'large']" >> "$GITHUB_OUTPUT"
        echo "compute-xl=['self-hosted', 'ondemand', 'linux', 'type=m5.2xlarge']" >> "$GITHUB_OUTPUT"
        ;;
    *)
        # shellcheck disable=SC2129
        echo "compute-small=['custom', 'linux', 'small']" >> "$GITHUB_OUTPUT"
        echo "compute-medium=['custom', 'linux', 'medium']" >> "$GITHUB_OUTPUT"
        echo "compute-large=['custom', 'linux', 'large']" >> "$GITHUB_OUTPUT"
        echo "compute-xl=['custom', 'linux', 'xl']" >> "$GITHUB_OUTPUT"
        ;;
esac
