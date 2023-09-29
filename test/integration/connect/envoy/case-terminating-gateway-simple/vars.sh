#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


# There is no sidecar proxy for s2, since the terminating gateway acts as the proxy
export REQUIRED_SERVICES="s1 s1-sidecar-proxy s2 terminating-gateway-primary"
