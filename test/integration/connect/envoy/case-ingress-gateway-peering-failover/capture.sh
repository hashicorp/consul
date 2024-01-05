#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


snapshot_envoy_admin localhost:20000 ingress-gateway primary || true
snapshot_envoy_admin localhost:19001 s2 primary || true
snapshot_envoy_admin localhost:19002 s2 alpha || true
snapshot_envoy_admin localhost:19003 mesh-gateway alpha || true
