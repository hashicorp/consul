#!/bin/sh

# Uses: https://github.com/globusdigital/deep-copy
deep-copy -pointer-receiver \
  -o ./proxycfg.deepcopy.go \
  -type ConfigSnapshot \
  -type ConfigSnapshotUpstreams \
  -type configSnapshotConnectProxy \
  -type configSnapshotIngressGateway \
  -type configSnapshotMeshGateway \
  -type configSnapshotTerminatingGateway \
  -type PeerServersValue \
  ./
