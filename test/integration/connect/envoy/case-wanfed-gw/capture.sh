#!/bin/bash

snapshot_envoy_admin localhost:19000 mesh-gateway primary || true
snapshot_envoy_admin localhost:19001 mesh-gateway secondary || true
