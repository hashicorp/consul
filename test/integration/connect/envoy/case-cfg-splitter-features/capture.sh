#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


snapshot_envoy_admin localhost:19000 s1 primary || true
snapshot_envoy_admin localhost:19001 s2-v1 primary || true
snapshot_envoy_admin localhost:19002 s2-v2 primary || true
snapshot_envoy_admin localhost:19003 s2 primary || true
