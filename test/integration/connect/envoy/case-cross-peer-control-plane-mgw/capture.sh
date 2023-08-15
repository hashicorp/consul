#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


snapshot_envoy_admin localhost:19001 mesh-gateway primary || true
snapshot_envoy_admin localhost:19003 mesh-gateway alpha || true
