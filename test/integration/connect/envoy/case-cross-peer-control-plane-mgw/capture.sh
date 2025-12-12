#!/bin/bash
# Copyright IBM Corp. 2014, 2025
# SPDX-License-Identifier: BUSL-1.1


snapshot_envoy_admin localhost:19001 mesh-gateway primary || true
snapshot_envoy_admin localhost:19003 mesh-gateway alpha || true
