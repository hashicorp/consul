#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


snapshot_envoy_admin localhost:20000 ingress-gateway primary || true
