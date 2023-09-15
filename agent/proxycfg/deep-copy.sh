#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


readonly PACKAGE_DIR="$(dirname "${BASH_SOURCE[0]}")"
cd $PACKAGE_DIR

# Uses: https://github.com/globusdigital/deep-copy
deep-copy -pointer-receiver \
  -o ./proxycfg.deepcopy.go \
  -type ConfigSnapshot \
  -type ConfigSnapshotUpstreams \
  -type PeerServersValue \
  -type PeeringServiceValue \
  -type configSnapshotAPIGateway \
  -type configSnapshotConnectProxy \
  -type configSnapshotIngressGateway \
  -type configSnapshotMeshGateway \
  -type configSnapshotTerminatingGateway \
  ./
