#!/bin/bash
# Copyright IBM Corp. 2014, 2025
# SPDX-License-Identifier: BUSL-1.1


# There is no sidecar proxy for s2, since the terminating gateway acts as the proxy
export REQUIRED_SERVICES="s1 s1-sidecar-proxy s4 terminating-gateway-primary"
