#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


readonly PACKAGE_DIR="$(dirname "${BASH_SOURCE[0]}")"
cd $PACKAGE_DIR

# Uses: https://github.com/globusdigital/deep-copy
deep-copy \
  -pointer-receiver \
  -o ./catalog_schema.deepcopy.go \
  -type upstreamDownstream \
  ./
