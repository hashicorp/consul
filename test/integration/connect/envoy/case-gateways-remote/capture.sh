#!/bin/bash

snapshot_envoy_admin localhost:19000 s1 primary || true
snapshot_envoy_admin localhost:19001 s2 secondary || true
snapshot_envoy_admin localhost:19002 mesh-gateway primary || true
snapshot_envoy_admin localhost:19003 mesh-gateway secondary || true