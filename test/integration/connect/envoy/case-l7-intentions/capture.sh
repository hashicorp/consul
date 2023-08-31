#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


snapshot_envoy_admin localhost:19000 s1 primary || true
snapshot_envoy_admin localhost:19001 s2 || true
