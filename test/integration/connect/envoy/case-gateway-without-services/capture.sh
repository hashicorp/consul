#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


snapshot_envoy_admin localhost:19000 mesh-gateway primary || true
