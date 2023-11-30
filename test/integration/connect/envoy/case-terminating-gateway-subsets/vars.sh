#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


# There is no sidecar proxy for s2-v1, since the terminating gateway acts as the proxy
export REQUIRED_SERVICES="
s1 s1-sidecar-proxy
s2-v1
s3 s3-sidecar-proxy
terminating-gateway-primary
"
