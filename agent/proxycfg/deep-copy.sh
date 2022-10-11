#!/bin/sh

# Uses: https://github.com/globusdigital/deep-copy
deep-copy -pointer-receiver \
  -o ./proxycfg.deepcopy.go \
  -type ConfigSnapshot \
  -type ConfigSnapshotUpstreams \
  -type PeerServersValue \
  -type PeeringServiceValue \
  -type configSnapshotConnectProxy \
  -type configSnapshotIngressGateway \
  -type configSnapshotMeshGateway \
  -type configSnapshotTerminatingGateway \
  ./
