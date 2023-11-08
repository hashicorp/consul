#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


export DEFAULT_REQUIRED_SERVICES="s1 s1-sidecar-proxy s2 s2-sidecar-proxy"
export REQUIRED_SERVICES="${DEFAULT_REQUIRED_SERVICES}"
export REQUIRE_SECONDARY=0
export REQUIRE_PARTITIONS=0
export REQUIRE_PEERS=0
