#!/bin/bash
# Copyright IBM Corp. 2014, 2025
# SPDX-License-Identifier: BUSL-1.1


snapshot_envoy_admin localhost:19000 s1 primary || true
snapshot_envoy_admin localhost:19001 s2 || true
