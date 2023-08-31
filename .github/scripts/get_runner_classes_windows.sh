#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

#
# This script generates tag-sets that can be used as runs-on: values to select runners.

set -euo pipefail

case "$GITHUB_REPOSITORY" in
*-enterprise)
	# shellcheck disable=SC2129
	echo "compute-small=['self-hosted', 'windows', 'small']" >>"$GITHUB_OUTPUT"
	echo "compute-medium=['self-hosted', 'windows', 'medium']" >>"$GITHUB_OUTPUT"
	echo "compute-large=['self-hosted', 'windows', 'large']" >>"$GITHUB_OUTPUT"
	# m5d.8xlarge is equivalent to our xl custom runner in CE
	echo "compute-xl=['self-hosted', 'ondemand', 'windows', 'type=m6a.2xlarge']" >>"$GITHUB_OUTPUT"
	;;
*)
	# shellcheck disable=SC2129
	echo "compute-small=['windows-2019']" >>"$GITHUB_OUTPUT"
	echo "compute-medium=['windows-2019']" >>"$GITHUB_OUTPUT"
	echo "compute-large=['windows-2019']" >>"$GITHUB_OUTPUT"
	echo "compute-xl=['windows-2019']" >>"$GITHUB_OUTPUT"
	;;
esac
