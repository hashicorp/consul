#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# Capture Envoy logs
docker logs $(docker ps -q --filter name=envoy) > envoy.log

# Capture Consul logs
docker logs $(docker ps -q --filter name=consul) > consul.log

# Check network connectivity
echo "Network connectivity check:" > network.log
docker exec -it $(docker ps -q --filter name=envoy) netstat -tulpn >> network.log
docker exec -it $(docker ps -q --filter name=envoy) curl -v localhost:20000/stats >> network.log 2>&1

snapshot_envoy_admin localhost:20000 api-gateway primary || true 