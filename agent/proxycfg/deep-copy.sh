#!/usr/bin/env bash

readonly PACKAGE_DIR="$(dirname "${BASH_SOURCE[0]}")"
cd $PACKAGE_DIR

# Uses: https://github.com/globusdigital/deep-copy
deep-copy -pointer-receiver \
  -o ./proxycfg.deepcopy.go \
  -type ConfigSnapshot \
  -type ConfigSnapshotUpstreams \
  -type configSnapshotConnectProxy \
  -type configSnapshotIngressGateway \
  -type configSnapshotMeshGateway \
  -type configSnapshotTerminatingGateway \
  ./
